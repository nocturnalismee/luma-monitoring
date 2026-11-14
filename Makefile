# LUMA Monitoring — root Makefile (created by task T-000)
# Targets documented in AGENT.md section 6. GO points at the toolchain
# required by docs/AGENT.md section 4 (Go >= 1.22); override with `make GO=go ...`.

GO      ?= go
NPM     ?= npm
BACKEND := backend
FRONTEND := frontend

.DEFAULT_GOAL := help

.PHONY: help setup dev-backend dev-frontend test lint build docker replay sync-agent

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

setup: ## Install backend and frontend dependencies
	cd $(BACKEND) && $(GO) mod download
	cd $(FRONTEND) && $(NPM) install

dev-backend: ## Run the backend with dev env (see .env.example)
	cd $(BACKEND) && $(GO) run ./cmd/luma

dev-frontend: ## Vite dev server (proxies /api to the backend)
	cd $(FRONTEND) && $(NPM) run dev

test: ## Backend tests (race) + frontend tests
	cd $(BACKEND) && $(GO) test -race ./...
	cd $(FRONTEND) && $(NPM) run test

lint: ## gofmt check, go vet, eslint, typecheck
	@unformatted=$$(cd $(BACKEND) && $(GO) fmt $$(cd $(BACKEND) && $(GO) list ./...) 2>/dev/null | sed 's|^$(BACKEND)/||'); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed on:"; echo "$$unformatted"; exit 1; fi
	cd $(BACKEND) && $(GO) vet ./...
	cd $(FRONTEND) && $(NPM) run lint
	cd $(FRONTEND) && $(NPM) run typecheck

build: ## Build frontend, then the Go binary
	cd $(FRONTEND) && $(NPM) run build
	cd $(BACKEND) && CGO_ENABLED=0 $(GO) build -o ../bin/luma ./cmd/luma

docker: ## Build the Docker image
	docker build -t luma-monitoring:latest .

replay: ## Send a fixture payload to the local backend: make replay FILE=backend/testdata/payloads/sample.body
	./scripts/replay.sh "$(FILE)"

sync-agent: ## Refresh agent distribution files from agent/ (used by api tasks, see DEPLOYMENT.md 4.2)
	rm -rf $(BACKEND)/internal/agentdist/files
	mkdir -p $(BACKEND)/internal/agentdist/files
	cp agent/luma_agent.sh agent/luma.cfg agent/luma_install.sh agent/luma_update.sh agent/luma_uninstall.sh $(BACKEND)/internal/agentdist/files/
