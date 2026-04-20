#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_SELF_HOSTED_KUBECONFIG="${HOME}/.kube/aods-self-hosted.yaml"

if [[ -f "${ROOT_DIR}/.envrc" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${ROOT_DIR}/.envrc"
  set +a
fi

KUBECONFIG_PATH="${AODS_K8S_KUBECONFIG:-${DEFAULT_SELF_HOSTED_KUBECONFIG}}"
HOSTNAME="${AODS_TESTBED_HOST:-aods.jalju.com}"
PLATFORM="${AODS_TESTBED_IMAGE_PLATFORM:-linux/amd64}"
STAMP="$(date +%Y%m%d%H%M%S)"

kubectl_args=(--kubeconfig "${KUBECONFIG_PATH}")
if [[ -n "${AODS_K8S_CONTEXT:-}" ]]; then
  kubectl_args+=(--context "${AODS_K8S_CONTEXT}")
fi

GHCR_USERNAME="${AODS_GHCR_USERNAME:-$(gh api user -q .login)}"
GHCR_TOKEN="${AODS_GHCR_TOKEN:-$(gh auth token)}"

BACKEND_IMAGE="${AODS_TESTBED_BACKEND_IMAGE:-ghcr.io/aolda/aods-backend:${STAMP}}"
FRONTEND_IMAGE="${AODS_TESTBED_FRONTEND_IMAGE:-ghcr.io/aolda/aods-frontend:${STAMP}}"

if [[ -z "${AODS_GIT_REMOTE:-}" ]]; then
  echo "AODS_GIT_REMOTE must be set before deploying." >&2
  exit 1
fi

if [[ -z "${GHCR_TOKEN}" ]]; then
  echo "AODS_GHCR_TOKEN or gh auth token must be available before deploying." >&2
  exit 1
fi

cd "${ROOT_DIR}"

echo "${GHCR_TOKEN}" | docker login ghcr.io -u "${GHCR_USERNAME}" --password-stdin

echo "Building backend image ${BACKEND_IMAGE}"
docker build --platform "${PLATFORM}" -f backend/Dockerfile -t "${BACKEND_IMAGE}" backend
docker push "${BACKEND_IMAGE}"

echo "Building frontend image ${FRONTEND_IMAGE}"
docker build --platform "${PLATFORM}" \
  --build-arg VITE_API_BASE_URL=/api \
  --build-arg VITE_AODS_PLATFORM_ADMIN_AUTHORITIES="${VITE_AODS_PLATFORM_ADMIN_AUTHORITIES:-/Ajou_Univ/Aolda_Admin,aods:platform:admin}" \
  -f frontend/Dockerfile \
  -t "${FRONTEND_IMAGE}" \
  frontend
docker push "${FRONTEND_IMAGE}"

kubectl "${kubectl_args[@]}" create namespace aods-system --dry-run=client -o yaml | kubectl "${kubectl_args[@]}" apply -f -

kubectl "${kubectl_args[@]}" -n aods-system create secret generic aods-backend-secrets \
  --from-literal=AODS_GIT_REMOTE="${AODS_GIT_REMOTE}" \
  --dry-run=client -o yaml | kubectl "${kubectl_args[@]}" apply -f -

kubectl "${kubectl_args[@]}" -n aods-system create secret docker-registry aods-registry-creds \
  --docker-server=ghcr.io \
  --docker-username="${GHCR_USERNAME}" \
  --docker-password="${GHCR_TOKEN}" \
  --dry-run=client -o yaml | kubectl "${kubectl_args[@]}" apply -f -

RENDERED_MANIFEST="$(mktemp)"
kubectl kustomize "${ROOT_DIR}/deploy/aods-system/overlays/testbed" \
  | sed "s|ghcr.io/aolda/aods-backend:latest|${BACKEND_IMAGE}|g; s|ghcr.io/aolda/aods-frontend:latest|${FRONTEND_IMAGE}|g" \
  > "${RENDERED_MANIFEST}"
kubectl "${kubectl_args[@]}" apply -f "${RENDERED_MANIFEST}"
rm -f "${RENDERED_MANIFEST}"

python3 - <<'PY' "${KUBECONFIG_PATH}" "${AODS_K8S_CONTEXT:-}" "${HOSTNAME}"
import json
import subprocess
import sys

kubeconfig, context, host = sys.argv[1], sys.argv[2], sys.argv[3]
cmd = [
    "kubectl",
    "--kubeconfig",
    kubeconfig,
]
if context:
    cmd += ["--context", context]
cmd += [
    "get",
    "gateway",
    "public-gateway",
    "-n",
    "istio-ingress",
    "-o",
    "json",
]
gateway = json.loads(subprocess.check_output(cmd, text=True))
changed = False
for server in gateway["spec"]["servers"]:
    hosts = server.setdefault("hosts", [])
    if host not in hosts:
      hosts.append(host)
      changed = True
if changed:
    patch = json.dumps({"spec": gateway["spec"]})
    subprocess.check_call([
        "kubectl",
        "--kubeconfig",
        kubeconfig,
        *([] if not context else ["--context", context]),
        "patch",
        "gateway",
        "public-gateway",
        "-n",
        "istio-ingress",
        "--type=merge",
        "-p",
        patch,
    ])
PY

kubectl "${kubectl_args[@]}" -n aods-system rollout status deployment/aods-backend --timeout=180s
kubectl "${kubectl_args[@]}" -n aods-system rollout status deployment/aods-frontend --timeout=180s

echo ""
echo "Backend image:  ${BACKEND_IMAGE}"
echo "Frontend image: ${FRONTEND_IMAGE}"
echo "URL: https://${HOSTNAME}"
