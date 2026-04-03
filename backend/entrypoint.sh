#!/bin/sh

set -eu

SERVICE_ACCOUNT_DIR="/var/run/secrets/kubernetes.io/serviceaccount"

if [ -z "${AODS_K8S_BEARER_TOKEN:-}" ] && [ -f "${SERVICE_ACCOUNT_DIR}/token" ]; then
  export AODS_K8S_BEARER_TOKEN="$(cat "${SERVICE_ACCOUNT_DIR}/token")"
fi

if [ -z "${AODS_K8S_CA_FILE:-}" ] && [ -f "${SERVICE_ACCOUNT_DIR}/ca.crt" ]; then
  export AODS_K8S_CA_FILE="${SERVICE_ACCOUNT_DIR}/ca.crt"
fi

exec /usr/local/bin/aods-backend
