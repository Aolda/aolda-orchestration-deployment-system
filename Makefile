.PHONY: backend-run frontend-run check setup

setup:
	@echo "Setting up basic directories..."
	mkdir -p backend/cmd/server backend/internal frontend scripts

backend-run:
	@echo "Running backend server..."
	@cd backend && go run cmd/server/main.go

frontend-run:
	@echo "Running frontend locally..."
	@cd frontend && npm run dev

check:
	@echo "Running checks for frontend and backend..."
	@cd backend && go vet ./... || true
	@cd frontend && npm run lint || true
