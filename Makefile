.PHONY: backend-run frontend-run check check-backend check-frontend check-manifests setup doctor install-self-hosted-kubeconfig deploy-testbed

setup:
	@echo "Setting up basic directories..."
	mkdir -p backend/cmd/server backend/internal frontend scripts config

backend-run:
	@echo "Running backend server..."
	@bash scripts/backend-run.sh

frontend-run:
	@echo "Running frontend locally..."
	@cd frontend && npm run dev

check:
	@echo "Running full validation for backend, manifests, and frontend..."
	@$(MAKE) check-backend
	@$(MAKE) check-manifests
	@$(MAKE) check-frontend

check-backend:
	@echo "Checking backend..."
	@bash scripts/check-backend.sh

check-frontend:
	@echo "Checking frontend..."
	@cd frontend && npm run check

check-manifests:
	@echo "Checking generated Kubernetes manifests..."
	@bash scripts/validate-manifests.sh

doctor:
	@bash scripts/doctor.sh

install-self-hosted-kubeconfig:
	@bash scripts/install-self-hosted-kubeconfig.sh

deploy-testbed:
	@bash scripts/deploy-testbed.sh
