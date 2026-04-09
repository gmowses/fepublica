.PHONY: help build test lint tidy dev up down logs migrate clean verify-cli docker-build

APP=fepublica
GO=go
GOFLAGS=-trimpath
LDFLAGS=-s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build all binaries to bin/
	@mkdir -p bin
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/collector ./cmd/collector
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/anchor ./cmd/anchor
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/api ./cmd/api
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/verify ./cmd/verify

test: ## Run unit tests with race detector
	$(GO) test -race -count=1 ./...

test-cover: ## Run tests with coverage report
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

lint: ## Run golangci-lint
	golangci-lint run ./...

tidy: ## go mod tidy
	$(GO) mod tidy

dev: ## Run local dev stack (postgres + api + anchor + collector)
	docker compose --profile dev up --build

up: ## Start docker compose stack
	docker compose up -d

down: ## Stop docker compose stack
	docker compose down

logs: ## Tail docker compose logs
	docker compose logs -f --tail=100

migrate: ## Run database migrations
	docker compose run --rm migrate

verify-cli: ## Build the standalone verify CLI only
	@mkdir -p bin
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/fepublica-verify ./cmd/verify

docker-build: ## Build docker image
	docker build -t $(APP):dev .

clean: ## Remove build artifacts
	rm -rf bin dist coverage.out
