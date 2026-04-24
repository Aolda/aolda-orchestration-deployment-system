#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

run_step() {
  local title="$1"
  shift
  printf '\n==> %s\n' "${title}"
  "$@"
}

check_backend_routes() {
  cd "${ROOT_DIR}/backend"
  go test ./internal/server \
    -run 'Test(CreateRedeployAndObserveApplication|GitModeCreateAndRedeployApplication)$' \
    -count=1
}

check_backend_domains() {
  cd "${ROOT_DIR}/backend"
  go test ./internal/application ./internal/vault -count=1
}

check_frontend_lint() {
  cd "${ROOT_DIR}/frontend"
  npm run lint
}

check_frontend_build() {
  cd "${ROOT_DIR}/frontend"
  npm run build
}

run_step "Backend API and GitOps regression tests: PrometheusRule, project health, metrics diagnostics" \
  check_backend_routes

run_step "Backend domain compile guard: application/vault observability-adjacent contracts" \
  check_backend_domains

run_step "Manifest validation: generated ServiceMonitor and PrometheusRule resources" \
  bash "${ROOT_DIR}/scripts/validate-manifests.sh"

run_step "Frontend API integration lint: project health snapshot client usage" \
  check_frontend_lint

run_step "Frontend API integration build: health snapshot and diagnostics types" \
  check_frontend_build

printf '\nObservability verification passed.\n'
