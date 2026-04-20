#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
COVERAGE_MIN="${AODS_BACKEND_COVERAGE_MIN:-32}"
ENABLE_RACE="${AODS_BACKEND_ENABLE_RACE:-1}"
PROFILE_PATH="$(mktemp "${TMPDIR:-/tmp}/aods-backend-cover.XXXXXX")"

cleanup() {
  rm -f "${PROFILE_PATH}"
}

trap cleanup EXIT

printf 'Running backend vet...\n'
(
  cd "${BACKEND_DIR}"
  go vet ./...
)

printf 'Running backend tests with coverage...\n'
(
  cd "${BACKEND_DIR}"
  go test ./... -covermode=atomic -coverprofile="${PROFILE_PATH}"
)

COVERAGE_LINE="$(
  cd "${BACKEND_DIR}"
  go tool cover -func="${PROFILE_PATH}" | tail -1
)"
COVERAGE_VALUE="$(printf '%s\n' "${COVERAGE_LINE}" | awk '{gsub("%", "", $3); print $3}')"

printf 'Backend total coverage: %s%% (minimum %s%%)\n' "${COVERAGE_VALUE}" "${COVERAGE_MIN}"

if ! awk -v actual="${COVERAGE_VALUE}" -v required="${COVERAGE_MIN}" 'BEGIN { exit((actual + 0) >= (required + 0) ? 0 : 1) }'; then
  printf 'Backend coverage gate failed.\n' >&2
  exit 1
fi

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    printf '## Backend Validation\n\n'
    printf -- '- Total coverage: `%s%%`\n' "${COVERAGE_VALUE}"
    printf -- '- Minimum required: `%s%%`\n' "${COVERAGE_MIN}"
    if [[ "${ENABLE_RACE}" == "0" ]]; then
      printf -- '- Race detector: skipped\n'
    else
      printf -- '- Race detector: enabled\n'
    fi
  } >> "${GITHUB_STEP_SUMMARY}"
fi

if [[ "${ENABLE_RACE}" == "0" ]]; then
  printf 'Skipping backend race detector.\n'
  exit 0
fi

printf 'Running backend race detector...\n'
(
  cd "${BACKEND_DIR}"
  go test -race ./...
)
