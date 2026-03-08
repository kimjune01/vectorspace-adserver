# CloudX Ad Server — Build & Deploy

.PHONY: dev staging prod test build-server build-portal build-landing clean deploy test-skills test-skill grade-skill

# ── Development ──────────────────────────────────────────────────

dev: ## Start dev server (in-memory DB, seeded, sidecar on 8081)
	go run ./cmd/server/ -db-path=vectorspace-dev.db -seed -sidecar-url=http://localhost:8081

dev-portal: ## Start portal dev server
	cd portal && pnpm dev

# ── Testing ──────────────────────────────────────────────────────

test: ## Run all Go tests
	go test ./... -count=1

test-portal: ## Type-check and build portal
	cd portal && npx tsc --noEmit && npx vite build

test-skills: ## Run all skill trials against cached repos
	bash skill/test/run.sh --grade

test-skill: ## Run one trial (REPO=owner/name SKILL=evaluate|install|verify)
	bash skill/test/run.sh --repo $(REPO) --skill $(SKILL)

grade-skill: ## Run one trial with grading (REPO=owner/name SKILL=evaluate|install|verify)
	bash skill/test/run.sh --repo $(REPO) --skill $(SKILL) --grade

# ── Build ────────────────────────────────────────────────────────

build-server: ## Build Go server binary
	CGO_ENABLED=0 go build -o bin/vectorspace-server ./cmd/server/

build-portal-staging: ## Build portal for staging
	cd portal && npx vite build --mode staging

build-portal-prod: ## Build portal for production
	cd portal && npx vite build --mode production

build-landing: ## Build landing site (Astro)
	cd landing && pnpm build

build: build-server build-portal-prod build-landing ## Build everything for production

# ── Staging ──────────────────────────────────────────────────────

staging: build-server build-portal-staging ## Build for staging
	@echo "Staging build complete:"
	@echo "  Server: bin/vectorspace-server"
	@echo "  Portal: portal/dist/"
	@echo ""
	@echo "Run: ./bin/vectorspace-server -db-path=vectorspace-staging.db -seed"

# ── Production ───────────────────────────────────────────────────

prod: build ## Build for production
	@echo "Production build complete:"
	@echo "  Server: bin/vectorspace-server"
	@echo "  Portal: portal/dist/"
	@echo ""
	@echo "Run: ./bin/vectorspace-server -db-path=/var/lib/vectorspace/prod.db"

# ── Docker ───────────────────────────────────────────────────────

docker-build: ## Build Docker image
	docker build -t vectorspace .

docker-run-dev: ## Run in Docker (dev)
	docker run -p 8080:8080 -e ANTHROPIC_API_KEY vectorspace -db-path=/data/vectorspace.db -seed

# ── Deploy ───────────────────────────────────────────────────────

deploy: ## Deploy infrastructure to AWS
	cd infra && set -a && . ./.env && set +a && pulumi up --stack dev

# ── Clean ────────────────────────────────────────────────────────

clean: ## Remove build artifacts
	rm -rf bin/ portal/dist/ landing/dist/

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
