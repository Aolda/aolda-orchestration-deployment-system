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

export AODS_IIV_ADDR="${AODS_IIV_ADDR:-${AODS_VAULT_ADDR:-}}"
export AODS_IIV_TOKEN="${AODS_IIV_TOKEN:-${AODS_VAULT_TOKEN:-}}"
export AODS_IIV_NAMESPACE="${AODS_IIV_NAMESPACE:-${AODS_VAULT_NAMESPACE:-}}"
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
if [[ -n "${AODS_IIV_ADDR:-}" ]]; then
  export AODS_IIV_ADDR="$(normalize_iiv_addr "${AODS_IIV_ADDR}")"
fi
if [[ -z "${AODS_SECRET_STORE_MODE:-}" && -n "${AODS_IIV_ADDR:-}${AODS_IIV_TOKEN:-}" ]]; then
  export AODS_SECRET_STORE_MODE=iiv
fi

# Background workers share the managed Git repo lock with user-facing catalog and
# application API paths. Keep heavy maintenance/sync loops opt-in for local runs
# so login bootstrap stays responsive while dogfooding.
if [[ -z "${AODS_ORPHAN_FLUX_CLEANUP_INTERVAL:-}" ]]; then
  export AODS_ORPHAN_FLUX_CLEANUP_INTERVAL=0
fi
if [[ -z "${AODS_REPOSITORY_POLL_INTERVAL:-}" ]]; then
  export AODS_REPOSITORY_POLL_INTERVAL=0
fi
if [[ -z "${AODS_APPLICATION_CATALOG_SYNC_INTERVAL:-}" ]]; then
  export AODS_APPLICATION_CATALOG_SYNC_INTERVAL=0
fi

if [[ -z "${AODS_K8S_KUBECONFIG:-}" && -f "${DEFAULT_SELF_HOSTED_KUBECONFIG}" ]]; then
  export AODS_K8S_KUBECONFIG="${DEFAULT_SELF_HOSTED_KUBECONFIG}"
fi

kubectl_args=()
if [[ -n "${AODS_K8S_KUBECONFIG:-}" ]]; then
  kubectl_args+=(--kubeconfig "${AODS_K8S_KUBECONFIG}")
fi
if [[ -n "${AODS_K8S_CONTEXT:-}" ]]; then
  kubectl_args+=(--context "${AODS_K8S_CONTEXT}")
fi

is_local_prometheus_url() {
  [[ "${AODS_PROMETHEUS_URL:-}" =~ ^http://(127\.0\.0\.1|localhost):[0-9]+$ ]]
}

is_local_iiv_url() {
  [[ "${AODS_IIV_ADDR:-}" =~ ^http://(127\.0\.0\.1|localhost):[0-9]+$ ]]
}

ensure_prometheus_port_forward() {
  if [[ "${AODS_PROMETHEUS_MODE:-local}" != "prometheus" ]]; then
    return
  fi

  if [[ -z "${AODS_PROMETHEUS_URL:-}" ]]; then
    echo "AODS_PROMETHEUS_URL is required when AODS_PROMETHEUS_MODE=prometheus." >&2
    exit 1
  fi

  if curl -fsS "${AODS_PROMETHEUS_URL}/-/healthy" >/dev/null 2>&1; then
    return
  fi

  if ! is_local_prometheus_url; then
    echo "Prometheus health check failed for ${AODS_PROMETHEUS_URL}. Configure a reachable URL or use a localhost URL for auto port-forward." >&2
    exit 1
  fi

  if [[ ${#kubectl_args[@]} -eq 0 ]]; then
    echo "AODS_K8S_KUBECONFIG must be set to auto-start the Prometheus port-forward." >&2
    exit 1
  fi

  local namespace="${AODS_PROMETHEUS_PORT_FORWARD_NAMESPACE:-monitoring}"
  local service="${AODS_PROMETHEUS_PORT_FORWARD_SERVICE:-kube-prometheus-stack-prometheus}"
  local local_port="${AODS_PROMETHEUS_PORT_FORWARD_LOCAL_PORT:-19090}"
  local remote_port="${AODS_PROMETHEUS_PORT_FORWARD_REMOTE_PORT:-9090}"
  local logfile="${AODS_PROMETHEUS_PORT_FORWARD_LOG:-/tmp/aods-prometheus-port-forward.log}"

  kubectl "${kubectl_args[@]}" -n "${namespace}" port-forward "svc/${service}" "${local_port}:${remote_port}" >"${logfile}" 2>&1 &
  local port_forward_pid=$!

  for _ in $(seq 1 20); do
    if curl -fsS "${AODS_PROMETHEUS_URL}/-/healthy" >/dev/null 2>&1; then
      echo "Prometheus port-forward ready on ${AODS_PROMETHEUS_URL} (pid=${port_forward_pid})"
      return
    fi
    sleep 0.5
  done

  echo "Failed to establish Prometheus port-forward. Check ${logfile}." >&2
  exit 1
}

ensure_iiv_port_forward() {
  if [[ "${AODS_SECRET_STORE_MODE:-local}" == "local" ]]; then
    return
  fi

  if [[ -z "${AODS_IIV_ADDR:-}" ]]; then
    echo "AODS_IIV_ADDR is required when AODS_SECRET_STORE_MODE=iiv." >&2
    exit 1
  fi

  if [[ -z "${AODS_IIV_TOKEN:-}" ]]; then
    echo "AODS_IIV_TOKEN is required when AODS_SECRET_STORE_MODE=iiv." >&2
    exit 1
  fi

  if curl -fsS -H "X-Vault-Token: ${AODS_IIV_TOKEN}" "${AODS_IIV_ADDR%/}/v1/sys/health" >/dev/null 2>&1; then
    return
  fi

  if [[ "${AODS_IIV_SKIP_HEALTH_CHECK:-${AODS_VAULT_SKIP_HEALTH_CHECK:-0}}" == "1" ]]; then
    echo "IIV health check skipped for ${AODS_IIV_ADDR}."
    return
  fi

  if ! is_local_iiv_url; then
    echo "IIV health check failed for ${AODS_IIV_ADDR}. Configure a reachable URL or use a localhost URL for auto port-forward." >&2
    exit 1
  fi

  if [[ ${#kubectl_args[@]} -eq 0 ]]; then
    echo "AODS_K8S_KUBECONFIG must be set to auto-start the IIV port-forward." >&2
    exit 1
  fi

  local namespace="${AODS_IIV_PORT_FORWARD_NAMESPACE:-${AODS_VAULT_PORT_FORWARD_NAMESPACE:-vault}}"
  local service="${AODS_IIV_PORT_FORWARD_SERVICE:-${AODS_VAULT_PORT_FORWARD_SERVICE:-vault}}"
  local local_port="${AODS_IIV_PORT_FORWARD_LOCAL_PORT:-${AODS_VAULT_PORT_FORWARD_LOCAL_PORT:-18200}}"
  local remote_port="${AODS_IIV_PORT_FORWARD_REMOTE_PORT:-${AODS_VAULT_PORT_FORWARD_REMOTE_PORT:-8200}}"
  local logfile="${AODS_IIV_PORT_FORWARD_LOG:-/tmp/aods-iiv-port-forward.log}"

  kubectl "${kubectl_args[@]}" -n "${namespace}" port-forward "svc/${service}" "${local_port}:${remote_port}" >"${logfile}" 2>&1 &
  local port_forward_pid=$!

  for _ in $(seq 1 20); do
    if curl -fsS -H "X-Vault-Token: ${AODS_IIV_TOKEN}" "${AODS_IIV_ADDR%/}/v1/sys/health" >/dev/null 2>&1; then
      echo "IIV port-forward ready on ${AODS_IIV_ADDR} (pid=${port_forward_pid})"
      return
    fi
    sleep 0.5
  done

  echo "Failed to establish IIV port-forward. Check ${logfile}." >&2
  exit 1
}

ensure_prometheus_port_forward
ensure_iiv_port_forward

cd "${ROOT_DIR}/backend"
exec go run cmd/server/main.go
