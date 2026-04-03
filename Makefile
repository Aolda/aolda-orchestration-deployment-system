.PHONY: backend-run frontend-run check setup doctor

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
	@echo "Running checks for frontend and backend..."
	@cd backend && go vet ./...
	@cd frontend && npm run lint

doctor:
	@bash scripts/doctor.sh
