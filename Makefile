# CloudX Ad Server — Build & Deploy

.PHONY: dev staging prod test build-server build-portal clean

# ── Development ──────────────────────────────────────────────────

dev: ## Start dev server (in-memory DB, seeded, sidecar on 8081)
	go run ./cmd/server/ -db-path=cloudx-dev.db -seed -sidecar-url=http://localhost:8081

dev-portal: ## Start portal dev server
	cd portal && pnpm dev

# ── Testing ──────────────────────────────────────────────────────

test: ## Run all Go tests
	go test ./... -count=1

test-portal: ## Type-check and build portal
	cd portal && npx tsc --noEmit && npx vite build

# ── Build ────────────────────────────────────────────────────────

build-server: ## Build Go server binary
	CGO_ENABLED=0 go build -o bin/cloudx-server ./cmd/server/

build-portal-staging: ## Build portal for staging
	cd portal && npx vite build --mode staging

build-portal-prod: ## Build portal for production
	cd portal && npx vite build --mode production

build: build-server build-portal-prod ## Build everything for production

# ── Staging ──────────────────────────────────────────────────────

staging: build-server build-portal-staging ## Build for staging
	@echo "Staging build complete:"
	@echo "  Server: bin/cloudx-server"
	@echo "  Portal: portal/dist/"
	@echo ""
	@echo "Run: ./bin/cloudx-server -db-path=cloudx-staging.db -seed"

# ── Production ───────────────────────────────────────────────────

prod: build ## Build for production
	@echo "Production build complete:"
	@echo "  Server: bin/cloudx-server"
	@echo "  Portal: portal/dist/"
	@echo ""
	@echo "Run: ./bin/cloudx-server -db-path=/var/lib/cloudx/prod.db"

# ── Docker ───────────────────────────────────────────────────────

docker-build: ## Build Docker image
	docker build -t cloudx-adserver .

docker-run-dev: ## Run in Docker (dev)
	docker run -p 8080:8080 -e ANTHROPIC_API_KEY cloudx-adserver -db-path=/data/cloudx.db -seed

# ── Clean ────────────────────────────────────────────────────────

clean: ## Remove build artifacts
	rm -rf bin/ portal/dist/

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
