#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/aods-manifests.XXXXXX")"
FIXTURE_ROOT="${TEMP_DIR}/generated-repo"
RENDER_DIR="${TEMP_DIR}/rendered"
SCHEMA_TEMPLATE="${ROOT_DIR}/ci/kubeconform-schemas/{{.ResourceKind}}{{.KindSuffix}}.json"
SERVER_MODE="${AODS_MANIFEST_SERVER_DRY_RUN:-auto}"
REQUIRE_SERVER="${AODS_MANIFEST_REQUIRE_SERVER:-0}"
KUBECONFORM_BIN="${KUBECONFORM_BIN:-kubeconform}"
KUBECONFORM_VERSION="${AODS_KUBECONFORM_VERSION:-v0.4.13}"

cleanup() {
  rm -rf "${TEMP_DIR}"
}

trap cleanup EXIT

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    printf 'required command not found: %s\n' "${cmd}" >&2
    exit 1
  fi
}

ensure_kubeconform() {
  if command -v "${KUBECONFORM_BIN}" >/dev/null 2>&1; then
    return 0
  fi

  if [[ "${KUBECONFORM_BIN}" != "kubeconform" ]]; then
    printf 'required command not found: %s\n' "${KUBECONFORM_BIN}" >&2
    exit 1
  fi

  local gopath fallback_bin
  gopath="$(go env GOPATH)"
  fallback_bin="${gopath}/bin/kubeconform"

  if [[ -x "${fallback_bin}" ]]; then
    KUBECONFORM_BIN="${fallback_bin}"
    return 0
  fi

  printf 'kubeconform not found; installing %s...\n' "${KUBECONFORM_VERSION}"
  (
    cd "${BACKEND_DIR}"
    go install "github.com/yannh/kubeconform/cmd/kubeconform@${KUBECONFORM_VERSION}"
  )

  if [[ ! -x "${fallback_bin}" ]]; then
    printf 'failed to install kubeconform to %s\n' "${fallback_bin}" >&2
    exit 1
  fi

  KUBECONFORM_BIN="${fallback_bin}"
}

sanitize_name() {
  printf '%s' "$1" | tr '/:' '__' | tr -cd '[:alnum:]_.-'
}

render_kustomization_tree() {
  local source_root="$1"
  local prefix="$2"
  local count=0

  while IFS= read -r kustomization; do
    local dir relative output
    dir="$(dirname "${kustomization}")"
    relative="${dir#${source_root}/}"
    output="${RENDER_DIR}/${prefix}-$(sanitize_name "${relative}").yaml"

    printf 'Rendering %s\n' "${dir#${ROOT_DIR}/}"
    kubectl kustomize "${dir}" > "${output}"
    count=$((count + 1))
  done < <(find "${source_root}" -name 'kustomization.yaml' -print | sort)

  if [[ "${count}" -eq 0 ]]; then
    printf 'No kustomizations found under %s\n' "${source_root#${ROOT_DIR}/}"
  fi
}

collect_namespaces() {
  awk '/^  namespace: / {print $2}' "${RENDER_DIR}"/*.yaml 2>/dev/null \
    | sed 's/^"//; s/"$//' \
    | sort -u
}

run_kubeconform() {
  ensure_kubeconform

  "${KUBECONFORM_BIN}" \
    -strict \
    -summary \
    -schema-location default \
    -schema-location "${SCHEMA_TEMPLATE}" \
    "${RENDER_DIR}"/*.yaml
}

server_validation_requested() {
  case "${SERVER_MODE}" in
    1|true|TRUE|yes|YES|on|ON)
      return 0
      ;;
    0|false|FALSE|no|NO|off|OFF)
      return 1
      ;;
    auto|AUTO)
      kubectl cluster-info >/dev/null 2>&1
      ;;
    *)
      printf 'unsupported AODS_MANIFEST_SERVER_DRY_RUN value: %s\n' "${SERVER_MODE}" >&2
      exit 1
      ;;
  esac
}

prepare_server_validation() {
  kubectl apply -f "${ROOT_DIR}/ci/crds/validation-crds.yaml" >/dev/null
  kubectl wait --for=condition=Established --timeout=60s \
    crd/kustomizations.kustomize.toolkit.fluxcd.io \
    crd/externalsecrets.external-secrets.io \
    crd/servicemonitors.monitoring.coreos.com \
    crd/prometheusrules.monitoring.coreos.com \
    crd/rollouts.argoproj.io \
    crd/virtualservices.networking.istio.io \
    crd/destinationrules.networking.istio.io >/dev/null

  while IFS= read -r namespace; do
    [[ -z "${namespace}" ]] && continue
    kubectl get namespace "${namespace}" >/dev/null 2>&1 || kubectl create namespace "${namespace}" >/dev/null
  done < <(collect_namespaces)
}

run_server_validation() {
  local rendered

  for rendered in "${RENDER_DIR}"/*.yaml; do
    printf 'Server dry-run %s\n' "$(basename "${rendered}")"
    kubectl apply --dry-run=server -f "${rendered}" >/dev/null
  done
}

require_cmd go
require_cmd kubectl

mkdir -p "${RENDER_DIR}"

printf 'Generating manifest fixtures...\n'
(
  cd "${BACKEND_DIR}"
  go run ./cmd/manifest-fixtures/main.go "${FIXTURE_ROOT}"
)

render_kustomization_tree "${ROOT_DIR}" "repo"
render_kustomization_tree "${FIXTURE_ROOT}" "generated"

printf 'Running schema validation on rendered manifests...\n'
run_kubeconform

if server_validation_requested; then
  printf 'Running Kubernetes server-side dry-run validation...\n'
  prepare_server_validation
  run_server_validation
elif [[ "${REQUIRE_SERVER}" == "1" ]]; then
  printf 'server dry-run validation was required but no Kubernetes cluster is reachable\n' >&2
  exit 1
else
  printf 'Skipping server-side dry-run validation because no Kubernetes cluster is reachable.\n'
fi
