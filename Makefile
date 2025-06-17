MODULE_IMAGE?=docker/docker-mcp-cli-desktop-module
MODULE := $(shell sh -c "awk '/^module/ { print \$$2 }' go.mod")
GO_VERSION := $(shell sh -c "awk '/^go / { print \$$2 }' go.mod")
GOLANGCI_LINT_VERSION := v2.1.6
GIT_TAG ?= $(shell git describe --tags --exact-match HEAD 2>/dev/null || git rev-parse HEAD)
GO_LDFLAGS = -X $(MODULE)/cmd/docker-mcp/version.Version=$(GIT_TAG)

export DOCKER_MCP_PLUGIN_BINARY := docker-mcp

ifeq ($(OS),Windows_NT)
	WINDOWS = $(OS)
	EXTENSION = .exe
	DOCKER_SHELL_CLI_PLUGIN_DIR = $(USERPROFILE)\.docker\cli-plugins
	DOCKER_MCP_CLI_PLUGIN_DST = $(DOCKER_SHELL_CLI_PLUGIN_DIR)\$(DOCKER_MCP_PLUGIN_BINARY)$(EXTENSION)
else
	WINDOWS =
	EXTENSION =
	DOCKER_SHELL_CLI_PLUGIN_DIR = $(HOME)/.docker/cli-plugins
	DOCKER_MCP_CLI_PLUGIN_DST = $(DOCKER_SHELL_CLI_PLUGIN_DIR)/$(DOCKER_MCP_PLUGIN_BINARY)$(EXTENSION)
endif

export GO_VERSION GO_LDFLAGS GOPRIVATE GOLANGCI_LINT_VERSION GIT_COMMIT GIT_TAG
DOCKER_BUILD_ARGS := --build-arg GO_VERSION \
					--build-arg GO_LDFLAGS \
          			--build-arg GOLANGCI_LINT_VERSION \
          			--build-arg DOCKER_MCP_PLUGIN_BINARY \

GO_TEST := go test
ifneq ($(shell sh -c "which gotestsum 2> /dev/null"),)
GO_TEST := gotestsum --format=testname --
endif

INFO_COLOR = \033[0;36m
NO_COLOR   = \033[m
LINT_PLATFORMS = linux,darwin,windows

format: ## Format code
	@docker buildx build $(DOCKER_BUILD_ARGS) -o . --target=format .

lint: ## Lint code
	@docker buildx build $(DOCKER_BUILD_ARGS) --pull --target=lint --platform=$(LINT_PLATFORMS) .

clean: ## remove built binaries and packages
	@sh -c "rm -rf bin dist"
	@sh -c "rm $(DOCKER_MCP_CLI_PLUGIN_DST)"

docker-mcp-cross:
	docker buildx build $(DOCKER_BUILD_ARGS) --pull --target=package-docker-mcp --platform=linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64 -o ./dist .

push-test-image: TAG=v100.0.8
push-test-image: MODULE_IMAGE=docker/docker-mcp-cli-desktop-module-test
push-test-image: docker-mcp-cross push-module-image ## push a test package

build-test-image: TAG=v100.0.8
build-test-image: MODULE_IMAGE=docker/docker-mcp-cli-desktop-module-test
build-test-image: docker-mcp-cross build-module-image ## create a test package

push-module-image: ## Build and push the image: make push-module-image TAG=v0.0.1
	cp -r dist ./module-image
	docker pull $(MODULE_IMAGE):$(TAG) && echo "Failure: Tag already exists" || docker buildx build --push --platform=linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64 --build-arg TAG=$(TAG) --tag=$(MODULE_IMAGE):$(TAG) ./module-image
	rm -rf ./module-image/dist

build-module-image: ## Build the image for the module: make build-module-image TAG=v0.0.1
	cp -r dist ./module-image
	docker buildx build --platform=linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64 --build-arg TAG=$(TAG) --output=docker --tag=$(MODULE_IMAGE):$(TAG) ./module-image
	rm -rf ./module-image/dist

mcp-package: ## package the server binaries
	tar -C dist/linux_amd64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-linux-amd64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY)
	tar -C dist/linux_arm64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-linux-arm64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY)
	tar -C dist/darwin_amd64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-darwin-amd64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY)
	tar -C dist/darwin_arm64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-darwin-arm64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY)
	tar -C dist/windows_amd64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-windows-amd64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY).exe
	tar -C dist/windows_arm64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-windows-arm64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY).exe

unit-tests:
	CGO_ENABLED=0 go test -v -tags="gen" ./...

docker-mcp:
	CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags "-s -w ${GO_LDFLAGS}" -o ./dist/$(DOCKER_MCP_PLUGIN_BINARY)$(EXTENSION) ./cmd/docker-mcp
	rm "$(DOCKER_MCP_CLI_PLUGIN_DST)" || true
	cp "dist/$(DOCKER_MCP_PLUGIN_BINARY)$(EXTENSION)" "$(DOCKER_MCP_CLI_PLUGIN_DST)"

help: ## Show this help
	@echo Please specify a build target. The choices are:
	@grep -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "$(INFO_COLOR)%-30s$(NO_COLOR) %s\n", $$1, $$2}'

.PHONY: run bin format lint unit-tests cross clean help docker-mcp
