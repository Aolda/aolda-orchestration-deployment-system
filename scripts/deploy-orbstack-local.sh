#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KUBECONFIG_MERGED="${HOME}/.kube/config:${HOME}/.orbstack/k8s/config.yml"
CONTEXT="orbstack"
NAMESPACE="aods-system"
BACKEND_IMAGE="aods-backend:orbstack-local"
FRONTEND_IMAGE="aods-frontend:orbstack-local"
FRONTEND_URL="http://localhost:18081"
BACKEND_URL="http://localhost:18080"

cd "${ROOT_DIR}"

orbctl start >/dev/null
orbctl start k8s >/dev/null

echo "Building backend image ${BACKEND_IMAGE}"
docker build -f backend/Dockerfile.local -t "${BACKEND_IMAGE}" .

echo "Building frontend image ${FRONTEND_IMAGE}"
docker build \
  --build-arg VITE_API_BASE_URL="${BACKEND_URL}" \
  --build-arg VITE_AODS_PLATFORM_ADMIN_AUTHORITIES="${VITE_AODS_PLATFORM_ADMIN_AUTHORITIES:-/Ajou_Univ/Aolda_Admin,aods:platform:admin}" \
  -f frontend/Dockerfile \
  -t "${FRONTEND_IMAGE}" \
  frontend

echo "Applying OrbStack overlay"
KUBECONFIG="${KUBECONFIG_MERGED}" kubectl --context "${CONTEXT}" apply -k deploy/aods-system/overlays/orbstack

echo "Waiting for rollouts"
KUBECONFIG="${KUBECONFIG_MERGED}" kubectl --context "${CONTEXT}" -n "${NAMESPACE}" rollout status deployment/aods-backend --timeout=180s
KUBECONFIG="${KUBECONFIG_MERGED}" kubectl --context "${CONTEXT}" -n "${NAMESPACE}" rollout status deployment/aods-frontend --timeout=180s

echo ""
echo "Frontend service:"
KUBECONFIG="${KUBECONFIG_MERGED}" kubectl --context "${CONTEXT}" -n "${NAMESPACE}" get svc aods-frontend
echo ""
echo "Backend service:"
KUBECONFIG="${KUBECONFIG_MERGED}" kubectl --context "${CONTEXT}" -n "${NAMESPACE}" get svc aods-backend
echo ""
echo "Next step:"
echo "  1. Port-forward backend:  KUBECONFIG=${KUBECONFIG_MERGED} kubectl --context ${CONTEXT} -n ${NAMESPACE} port-forward svc/aods-backend 18080:8080"
echo "  2. Port-forward frontend: KUBECONFIG=${KUBECONFIG_MERGED} kubectl --context ${CONTEXT} -n ${NAMESPACE} port-forward svc/aods-frontend 18081:80"
echo "  3. Open ${FRONTEND_URL}"
