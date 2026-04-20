#!/usr/bin/env bash

set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SELF_HOSTED_KUBECONFIG="${HOME}/.kube/aods-self-hosted.yaml"
status=0

if [ -f "${ROOT_DIR}/.envrc" ]; then
  set -a
  # shellcheck disable=SC1091
  source "${ROOT_DIR}/.envrc"
  set +a
fi

check_cmd() {
  local cmd="$1"
  if command -v "$cmd" >/dev/null 2>&1; then
    printf '[ok] %s found\n' "$cmd"
  else
    printf '[warn] %s not found\n' "$cmd"
    status=1
  fi
}

check_file() {
  local path="$1"
  if [ -f "$path" ]; then
    printf '[ok] %s exists\n' "$path"
  else
    printf '[warn] %s is missing\n' "$path"
    status=1
  fi
}

check_env() {
  local name="$1"
  if [ -n "${!name:-}" ]; then
    printf '[ok] %s is set\n' "$name"
  else
    printf '[warn] %s is not set\n' "$name"
    status=1
  fi
}

check_env_equals() {
  local name="$1"
  local expected="$2"
  if [ "${!name:-}" = "$expected" ]; then
    printf '[ok] %s=%s\n' "$name" "$expected"
  else
    printf '[warn] %s must be %s (current: %s)\n' "$name" "$expected" "${!name:-unset}"
    status=1
  fi
}

check_optional_file() {
  local path="$1"
  if [ -f "$path" ]; then
    printf '[ok] optional %s exists\n' "$path"
  else
    printf '[info] optional %s is not configured\n' "$path"
  fi
}

check_optional_env() {
  local name="$1"
  if [ -n "${!name:-}" ]; then
    printf '[ok] optional %s is set\n' "$name"
  else
    printf '[info] optional %s is not set\n' "$name"
  fi
}

check_kube_access() {
  if ! command -v kubectl >/dev/null 2>&1; then
    return
  fi

  if [ ! -f "${SELF_HOSTED_KUBECONFIG}" ]; then
    return
  fi

  if KUBECONFIG="${SELF_HOSTED_KUBECONFIG}" kubectl get nodes -o name >/dev/null 2>&1; then
    printf '[ok] self-hosted cluster is reachable via %s\n' "${SELF_HOSTED_KUBECONFIG}"
  else
    printf '[warn] self-hosted cluster is not reachable via %s\n' "${SELF_HOSTED_KUBECONFIG}"
    status=1
  fi
}

printf 'Harness setup check for AODS\n'

check_cmd git
check_cmd make
check_cmd go
check_cmd node
check_cmd npm
check_cmd kubectl
check_cmd curl

if command -v codex >/dev/null 2>&1; then
  printf '[ok] codex found\n'
elif command -v claude >/dev/null 2>&1; then
  printf '[ok] claude found\n'
else
  printf '[warn] neither codex nor claude was found\n'
  status=1
fi

check_file "${SELF_HOSTED_KUBECONFIG}"
check_env_equals AODS_GIT_MODE git
check_env AODS_GIT_REMOTE
check_env_equals AODS_K8S_MODE kubeconfig
check_env AODS_K8S_KUBECONFIG
check_env_equals AODS_PROMETHEUS_MODE prometheus
check_env AODS_PROMETHEUS_URL
check_env_equals AODS_VAULT_MODE token
check_env AODS_VAULT_ADDR
check_env AODS_VAULT_TOKEN
check_kube_access

check_optional_file config/mcporter.json
check_optional_env GCAL_CALENDAR_ID

if [ "$status" -eq 0 ]; then
  printf 'Real dev baseline looks ready.\n'
else
  printf 'Real dev baseline is incomplete.\n'
fi

exit "$status"
