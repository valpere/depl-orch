.PHONY: help all build test fmt lint dashboard e2e deployer review haiku clean

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

all: build ## Build the project

build: ## Compile all packages
	go build ./...

test: ## Run unit tests
	go test ./...

fmt: ## Format the code (gofmt, goimports)
	gofmt -w .
	goimports -w .

lint: ## Run go vet over all packages
	go vet ./...

dashboard: ## Start the Agentic Recovery Dashboard on port 8082
	go run ./cmd/dashboard

e2e: ## Run end-to-end integration tests
	go test -tags integration ./test/e2e/...

# Include .env if it exists
-include .env
export

deployer: ## Run the main deployer pipeline locally (with dummy image and push disabled)
	DEPLOY_IMAGE=$${DEPLOY_IMAGE:-local-test-image:latest} \
	DEPLOY_PUSH=$${DEPLOY_PUSH:-false} \
	go run ./cmd/deployer

review: ## Run independent review via OpenCode minimax-m3
	opencode run --agent reviewer

haiku: ## Run stateless Haiku tool lane
	claude -p --model haiku --output-format json --permission-mode acceptEdits --append-system-prompt --fallback-model sonnet

clean: ## Clean up transient scratch files
	rm -rf .agents/scratch/*
	find . -type d -name "_*" -prune -exec rm -rf {} +
	find . -type f -name "_*" -delete

