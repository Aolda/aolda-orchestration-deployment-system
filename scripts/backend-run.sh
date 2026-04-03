#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ -f "${ROOT_DIR}/.envrc" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${ROOT_DIR}/.envrc"
  set +a
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

ensure_prometheus_port_forward

cd "${ROOT_DIR}/backend"
exec go run cmd/server/main.go
