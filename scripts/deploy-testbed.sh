#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_SELF_HOSTED_KUBECONFIG="${HOME}/.kube/aods-self-hosted.yaml"
ENV_AODS_K8S_KUBECONFIG="${AODS_K8S_KUBECONFIG:-}"
ENV_AODS_K8S_CONTEXT="${AODS_K8S_CONTEXT:-}"

if [[ "${AODS_DEPLOY_TESTBED_SKIP_ENVRC:-}" != "1" && -f "${ROOT_DIR}/.envrc" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${ROOT_DIR}/.envrc"
  set +a
fi
if [[ -n "${ENV_AODS_K8S_KUBECONFIG}" ]]; then
  AODS_K8S_KUBECONFIG="${ENV_AODS_K8S_KUBECONFIG}"
fi
if [[ -n "${ENV_AODS_K8S_CONTEXT}" ]]; then
  AODS_K8S_CONTEXT="${ENV_AODS_K8S_CONTEXT}"
fi

KUBECONFIG_PATH="${AODS_K8S_KUBECONFIG:-${DEFAULT_SELF_HOSTED_KUBECONFIG}}"
HOSTNAME="${AODS_TESTBED_HOST:-aods.ajou.app}"
PLATFORM="${AODS_TESTBED_IMAGE_PLATFORM:-linux/amd64}"
STAMP="$(date +%Y%m%d%H%M%S)"

kubectl_args=(--kubeconfig "${KUBECONFIG_PATH}")
if [[ -n "${AODS_K8S_CONTEXT:-}" ]]; then
  kubectl_args+=(--context "${AODS_K8S_CONTEXT}")
fi

BACKEND_IMAGE="${AODS_TESTBED_BACKEND_IMAGE:-ghcr.io/aolda/aods-backend:${STAMP}}"
FRONTEND_IMAGE="${AODS_TESTBED_FRONTEND_IMAGE:-ghcr.io/aolda/aods-frontend:${STAMP}}"
REQUIRES_GHCR_AUTH=false
if [[ "${BACKEND_IMAGE}" == ghcr.io/* || "${FRONTEND_IMAGE}" == ghcr.io/* ]]; then
  REQUIRES_GHCR_AUTH=true
fi

if [[ -z "${AODS_GIT_REMOTE:-}" ]]; then
  echo "AODS_GIT_REMOTE must be set before deploying." >&2
  exit 1
fi

AODS_IIV_ADDR="${AODS_IIV_ADDR:-${AODS_VAULT_ADDR:-}}"
AODS_IIV_TOKEN="${AODS_IIV_TOKEN:-${AODS_VAULT_TOKEN:-}}"
AODS_IIV_NAMESPACE="${AODS_IIV_NAMESPACE:-${AODS_VAULT_NAMESPACE:-}}"

normalize_iiv_addr() {
  local value="$1"
  if [[ "${value}" == *://* ]]; then
    printf '%s' "${value}"
  elif [[ "${value}" == *:* ]]; then
    printf 'http://%s' "${value}"
  else
    printf 'http://%s:8200' "${value}"
  fi
}

if [[ -z "${AODS_IIV_ADDR:-}" ]]; then
  echo "AODS_IIV_ADDR must be set before deploying with the real IIV adapter." >&2
  exit 1
fi
AODS_IIV_ADDR="$(normalize_iiv_addr "${AODS_IIV_ADDR}")"

if [[ -z "${AODS_IIV_TOKEN:-}" ]]; then
  echo "AODS_IIV_TOKEN must be set before deploying with the real IIV adapter." >&2
  exit 1
fi

cd "${ROOT_DIR}"

if [[ "${REQUIRES_GHCR_AUTH}" == "true" ]]; then
  GHCR_USERNAME="${AODS_GHCR_USERNAME:-$(gh api user -q .login)}"
  GHCR_TOKEN="${AODS_GHCR_TOKEN:-$(gh auth token)}"

  if [[ -z "${GHCR_TOKEN}" ]]; then
    echo "AODS_GHCR_TOKEN or gh auth token must be available before deploying GHCR images." >&2
    exit 1
  fi

  echo "${GHCR_TOKEN}" | docker login ghcr.io -u "${GHCR_USERNAME}" --password-stdin
fi

echo "Building backend image ${BACKEND_IMAGE}"
docker build --platform "${PLATFORM}" -f backend/Dockerfile -t "${BACKEND_IMAGE}" backend
docker push "${BACKEND_IMAGE}"

echo "Building frontend image ${FRONTEND_IMAGE}"
docker build --platform "${PLATFORM}" \
  --build-arg VITE_API_BASE_URL=/api \
  --build-arg VITE_AODS_AUTH_MODE="${VITE_AODS_AUTH_MODE:-oidc}" \
  --build-arg VITE_AODS_OIDC_ISSUER_URL="${VITE_AODS_OIDC_ISSUER_URL:-https://sso.aoldacloud.com/realms/aolda}" \
  --build-arg VITE_AODS_OIDC_CLIENT_ID="${VITE_AODS_OIDC_CLIENT_ID:-aods}" \
  --build-arg VITE_AODS_OIDC_REDIRECT_URI="${VITE_AODS_OIDC_REDIRECT_URI:-https://${HOSTNAME}/}" \
  --build-arg VITE_AODS_OIDC_SCOPE="${VITE_AODS_OIDC_SCOPE:-openid profile email}" \
  --build-arg VITE_AODS_ALLOW_EMERGENCY_LOGIN="${VITE_AODS_ALLOW_EMERGENCY_LOGIN:-true}" \
  --build-arg VITE_AODS_PLATFORM_ADMIN_AUTHORITIES="${VITE_AODS_PLATFORM_ADMIN_AUTHORITIES:-/Ajou_Univ/Aolda_Admin,aods:platform:admin}" \
  -f frontend/Dockerfile \
  -t "${FRONTEND_IMAGE}" \
  frontend
docker push "${FRONTEND_IMAGE}"

kubectl "${kubectl_args[@]}" create namespace aods-system --dry-run=client -o yaml | kubectl "${kubectl_args[@]}" apply -f -

backend_secret_args=(
  --from-literal=AODS_GIT_REMOTE="${AODS_GIT_REMOTE}"
  --from-literal=AODS_SECRET_STORE_MODE="${AODS_SECRET_STORE_MODE:-iiv}"
  --from-literal=AODS_IIV_ADDR="${AODS_IIV_ADDR}"
  --from-literal=AODS_IIV_TOKEN="${AODS_IIV_TOKEN}"
)
if [[ -n "${AODS_IIV_NAMESPACE:-}" ]]; then
  backend_secret_args+=(--from-literal=AODS_IIV_NAMESPACE="${AODS_IIV_NAMESPACE}")
fi
if [[ -n "${AODS_MARIADB_DSN:-}" ]]; then
  backend_secret_args+=(--from-literal=AODS_MARIADB_DSN="${AODS_MARIADB_DSN}")
fi
if [[ -n "${AODS_APPLICATION_CATALOG_DSN:-}" ]]; then
  backend_secret_args+=(--from-literal=AODS_APPLICATION_CATALOG_DSN="${AODS_APPLICATION_CATALOG_DSN}")
fi
if [[ -n "${AODS_APPLICATION_CATALOG_DB_DRIVER:-}" ]]; then
  backend_secret_args+=(--from-literal=AODS_APPLICATION_CATALOG_DB_DRIVER="${AODS_APPLICATION_CATALOG_DB_DRIVER}")
fi
if [[ -n "${AODS_APPLICATION_CATALOG_CACHE_TTL:-}" ]]; then
  backend_secret_args+=(--from-literal=AODS_APPLICATION_CATALOG_CACHE_TTL="${AODS_APPLICATION_CATALOG_CACHE_TTL}")
fi
if [[ -n "${AODS_APPLICATION_CATALOG_SYNC_INTERVAL:-}" ]]; then
  backend_secret_args+=(--from-literal=AODS_APPLICATION_CATALOG_SYNC_INTERVAL="${AODS_APPLICATION_CATALOG_SYNC_INTERVAL}")
fi
kubectl "${kubectl_args[@]}" -n aods-system create secret generic aods-backend-secrets \
  "${backend_secret_args[@]}" \
  --dry-run=client -o yaml | kubectl "${kubectl_args[@]}" apply -f -

if kubectl "${kubectl_args[@]}" get crd clustersecretstores.external-secrets.io >/dev/null 2>&1; then
  cat <<YAML | kubectl "${kubectl_args[@]}" apply -f -
apiVersion: external-secrets.io/v1
kind: ClusterSecretStore
metadata:
  name: aods-iiv
spec:
  provider:
    vault:
      server: ${AODS_IIV_ADDR}
      path: secret
      version: v2
      auth:
        tokenSecretRef:
          name: aods-backend-secrets
          namespace: aods-system
          key: AODS_IIV_TOKEN
YAML
else
  echo "External Secrets ClusterSecretStore CRD is not installed; skipping aods-iiv ClusterSecretStore."
fi

if [[ "${REQUIRES_GHCR_AUTH}" == "true" ]]; then
  kubectl "${kubectl_args[@]}" -n aods-system create secret docker-registry aods-registry-creds \
    --docker-server=ghcr.io \
    --docker-username="${GHCR_USERNAME}" \
    --docker-password="${GHCR_TOKEN}" \
    --dry-run=client -o yaml | kubectl "${kubectl_args[@]}" apply -f -
fi

RENDERED_MANIFEST="$(mktemp)"
kubectl kustomize "${ROOT_DIR}/deploy/aods-system/overlays/testbed" \
  | sed "s|ghcr.io/aolda/aods-backend:latest|${BACKEND_IMAGE}|g; s|ghcr.io/aolda/aods-frontend:latest|${FRONTEND_IMAGE}|g" \
  > "${RENDERED_MANIFEST}"
if [[ "${REQUIRES_GHCR_AUTH}" != "true" ]]; then
  RENDERED_WITHOUT_PULL_SECRET="$(mktemp)"
  sed '/imagePullSecrets:/,+1d' "${RENDERED_MANIFEST}" > "${RENDERED_WITHOUT_PULL_SECRET}"
  mv "${RENDERED_WITHOUT_PULL_SECRET}" "${RENDERED_MANIFEST}"
fi
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
