#!/usr/bin/env bash

set -euo pipefail

SOURCE_PATH="${1:-/tmp/aods-self-hosted.kubeconfig}"
TARGET_PATH="${2:-${HOME}/.kube/aods-self-hosted.yaml}"

if [[ ! -f "${SOURCE_PATH}" ]]; then
  echo "kubeconfig not found: ${SOURCE_PATH}" >&2
  exit 1
fi

mkdir -p "$(dirname "${TARGET_PATH}")"
cp "${SOURCE_PATH}" "${TARGET_PATH}"
chmod 600 "${TARGET_PATH}"

echo "Installed self-hosted kubeconfig to ${TARGET_PATH}"
KUBECONFIG="${TARGET_PATH}" kubectl config get-contexts
