SHELL := /bin/sh

BINARY := bin/retire
MAIN   := ./cmd/retire

.DEFAULT_GOAL := help

help: ## List available targets
	@printf 'Usage: make <target>\n\nTargets:\n'
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the retire CLI into bin/
	@mkdir -p $(dir $(BINARY))
	go build -o $(BINARY) $(MAIN)

test: ## Run the full test suite
	go test ./...

test-race: ## Run the test suite with the race detector
	go test -race ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go source in place
	gofmt -w .

fmt-check: ## Fail if any Go source is unformatted
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

lint: ## Run golangci-lint
	golangci-lint run

tidy: ## Tidy go.mod and go.sum
	go mod tidy

run: ## Run the CLI, e.g. make run ARGS="project --config household.yaml"
	go run $(MAIN) $(ARGS)

check: fmt-check vet lint test ## Run every CI gate (fmt-check, vet, lint, test)

clean: ## Remove build artifacts
	rm -rf bin

.PHONY: help build test test-race vet fmt fmt-check lint tidy run check clean
