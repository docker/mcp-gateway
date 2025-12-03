.PHONY: help build push build-push run clean test lint

# Docker image configuration
IMAGE_NAME := jimclark106/vector-db
TAG ?= latest
PLATFORM ?= linux/amd64

# Local build configuration
BINARY_NAME := sqlite-vec-mcp
GO_BUILD_FLAGS := -tags "sqlite_extensions" -ldflags="-s -w"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build Docker image for linux/amd64
	docker buildx build \
		--platform linux/amd64 \
		-t $(IMAGE_NAME):$(TAG) \
		--load \
		.

build-multi: ## Build Docker image for multiple platforms
	docker buildx build \
		--platform $(PLATFORM) \
		-t $(IMAGE_NAME):$(TAG) \
		--load \
		.

build-push: ## Build and push Docker image to registry
	docker buildx build \
		--platform $(PLATFORM) \
		-t $(IMAGE_NAME):$(TAG) \
		--push \
		.

push: ## Push Docker image to registry
	docker push $(IMAGE_NAME):$(TAG)

run: ## Run the MCP server locally in Docker
	docker run --rm -i \
		--platform $(PLATFORM) \
		-v $(PWD)/data:/data \
		-e DB_PATH=/data/vectors.db \
		-e VECTOR_DIMENSION=1536 \
		$(IMAGE_NAME):$(TAG)

build-local: ## Build the binary locally (for testing)
	CGO_ENABLED=1 go build $(GO_BUILD_FLAGS) -o $(BINARY_NAME) main.go

run-local: build-local ## Build and run the binary locally
	./$(BINARY_NAME)

clean: ## Clean up built artifacts
	rm -f $(BINARY_NAME)
	rm -rf data/*.db
	docker rmi $(IMAGE_NAME):$(TAG) || true

test: ## Run tests
	go test -v ./...

lint: ## Run linters
	golangci-lint run ./...

fmt: ## Format code
	go fmt ./...
	gofmt -w main.go

deps: ## Download dependencies
	go mod download
	go mod tidy

.DEFAULT_GOAL := help
