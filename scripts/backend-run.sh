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

# The orphan Flux cleanup worker can monopolize the managed Git repo lock during
# local dogfooding and block the initial /projects bootstrap. Keep it opt-in for
# local runs; deployed environments can still enable it explicitly via env.
if [[ -z "${AODS_ORPHAN_FLUX_CLEANUP_INTERVAL:-}" ]]; then
  export AODS_ORPHAN_FLUX_CLEANUP_INTERVAL=0
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

is_local_vault_url() {
  [[ "${AODS_VAULT_ADDR:-}" =~ ^http://(127\.0\.0\.1|localhost):[0-9]+$ ]]
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

ensure_vault_port_forward() {
  if [[ "${AODS_VAULT_MODE:-local}" != "token" ]]; then
    return
  fi

  if [[ -z "${AODS_VAULT_ADDR:-}" ]]; then
    echo "AODS_VAULT_ADDR is required when AODS_VAULT_MODE=token." >&2
    exit 1
  fi

  if [[ -z "${AODS_VAULT_TOKEN:-}" ]]; then
    echo "AODS_VAULT_TOKEN is required when AODS_VAULT_MODE=token." >&2
    exit 1
  fi

  if curl -fsS -H "X-Vault-Token: ${AODS_VAULT_TOKEN}" "${AODS_VAULT_ADDR}/v1/sys/health" >/dev/null 2>&1; then
    return
  fi

  if ! is_local_vault_url; then
    echo "Vault health check failed for ${AODS_VAULT_ADDR}. Configure a reachable URL or use a localhost URL for auto port-forward." >&2
    exit 1
  fi

  if [[ ${#kubectl_args[@]} -eq 0 ]]; then
    echo "AODS_K8S_KUBECONFIG must be set to auto-start the Vault port-forward." >&2
    exit 1
  fi

  local namespace="${AODS_VAULT_PORT_FORWARD_NAMESPACE:-vault}"
  local service="${AODS_VAULT_PORT_FORWARD_SERVICE:-vault}"
  local local_port="${AODS_VAULT_PORT_FORWARD_LOCAL_PORT:-18200}"
  local remote_port="${AODS_VAULT_PORT_FORWARD_REMOTE_PORT:-8200}"
  local logfile="${AODS_VAULT_PORT_FORWARD_LOG:-/tmp/aods-vault-port-forward.log}"

  kubectl "${kubectl_args[@]}" -n "${namespace}" port-forward "svc/${service}" "${local_port}:${remote_port}" >"${logfile}" 2>&1 &
  local port_forward_pid=$!

  for _ in $(seq 1 20); do
    if curl -fsS -H "X-Vault-Token: ${AODS_VAULT_TOKEN}" "${AODS_VAULT_ADDR}/v1/sys/health" >/dev/null 2>&1; then
      echo "Vault port-forward ready on ${AODS_VAULT_ADDR} (pid=${port_forward_pid})"
      return
    fi
    sleep 0.5
  done

  echo "Failed to establish Vault port-forward. Check ${logfile}." >&2
  exit 1
}

ensure_prometheus_port_forward
ensure_vault_port_forward

cd "${ROOT_DIR}/backend"
exec go run cmd/server/main.go
