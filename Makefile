.PHONY: build test clean install run fmt vet lint help

# Build variables
BINARY_NAME=mcp-manager
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)"

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/mcp-manager

install: ## Install the binary to GOPATH/bin
	$(GOCMD) install $(LDFLAGS) ./cmd/mcp-manager

run: ## Run the application (with any args: make run ARGS="start --all")
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/mcp-manager
	./$(BINARY_NAME) $(ARGS)

test: ## Run tests
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

coverage: test ## Run tests and show coverage in browser
	$(GOCMD) tool cover -html=coverage.out

fmt: ## Format code
	$(GOFMT) ./...

vet: ## Run go vet
	$(GOVET) ./...

lint: ## Run golangci-lint (requires golangci-lint installed)
	golangci-lint run ./...

tidy: ## Tidy go modules
	$(GOMOD) tidy

clean: ## Remove build artifacts
	rm -f $(BINARY_NAME)
	rm -f coverage.out
	rm -rf dist/

deps: ## Download dependencies
	$(GOMOD) download

.DEFAULT_GOAL := help
