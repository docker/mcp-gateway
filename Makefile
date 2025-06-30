MODULE_IMAGE?=docker/docker-mcp-cli-desktop-module
MODULE := $(shell sh -c "awk '/^module/ { print \$$2 }' go.mod")
GO_VERSION := $(shell sh -c "awk '/^go / { print \$$2 }' go.mod")
GOLANGCI_LINT_VERSION ?= v2.1.6
GIT_TAG ?= $(shell git describe --tags --exact-match HEAD 2>/dev/null || git rev-parse HEAD)
GO_LDFLAGS = -X $(MODULE)/cmd/docker-mcp/version.Version=$(GIT_TAG)

export DOCKER_MCP_PLUGIN_BINARY ?= docker-mcp

ifeq ($(OS),Windows_NT)
	EXTENSION = .exe
	DOCKER_MCP_CLI_PLUGIN_DST = $(USERPROFILE)\.docker\cli-plugins\$(DOCKER_MCP_PLUGIN_BINARY)$(EXTENSION)
else
	EXTENSION =
	DOCKER_MCP_CLI_PLUGIN_DST = $(HOME)/.docker/cli-plugins/$(DOCKER_MCP_PLUGIN_BINARY)$(EXTENSION)
endif

export GO_VERSION GO_LDFLAGS GOPRIVATE GOLANGCI_LINT_VERSION GIT_COMMIT GIT_TAG
DOCKER_BUILD_ARGS := --build-arg GO_VERSION \
					--build-arg GO_LDFLAGS \
          			--build-arg GOLANGCI_LINT_VERSION \
          			--build-arg DOCKER_MCP_PLUGIN_BINARY

format:
	docker buildx build $(DOCKER_BUILD_ARGS) --target=format -o . .

lint:
	docker buildx build $(DOCKER_BUILD_ARGS) --target=lint --platform=linux,darwin,windows .

clean:
	@sh -c "rm -rf bin dist"
	@sh -c "rm $(DOCKER_MCP_CLI_PLUGIN_DST)"

docker-mcp-cross:
	docker buildx build $(DOCKER_BUILD_ARGS) --target=package-docker-mcp --platform=linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64 -o ./dist .

push-module-image:
	cp -r dist ./module-image
	docker buildx build --push --platform=linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64 --build-arg TAG=$(TAG) --tag=$(MODULE_IMAGE):$(TAG) ./module-image
	rm -rf ./module-image/dist

mcp-package:
	tar -C dist/linux_amd64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-linux-amd64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY)
	tar -C dist/linux_arm64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-linux-arm64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY)
	tar -C dist/darwin_amd64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-darwin-amd64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY)
	tar -C dist/darwin_arm64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-darwin-arm64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY)
	tar -C dist/windows_amd64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-windows-amd64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY).exe
	tar -C dist/windows_arm64 -czf dist/$(DOCKER_MCP_PLUGIN_BINARY)-windows-arm64.tar.gz $(DOCKER_MCP_PLUGIN_BINARY).exe

test:
	docker buildx build $(DOCKER_BUILD_ARGS) --target=test .

docker-mcp:
	CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags "-s -w ${GO_LDFLAGS}" -o ./dist/$(DOCKER_MCP_PLUGIN_BINARY)$(EXTENSION) ./cmd/docker-mcp
	rm "$(DOCKER_MCP_CLI_PLUGIN_DST)" || true
	cp "dist/$(DOCKER_MCP_PLUGIN_BINARY)$(EXTENSION)" "$(DOCKER_MCP_CLI_PLUGIN_DST)"

push-mcp-gateway:
	docker buildx bake mcp-gateway --push

push-l4proxy-image:
	docker buildx bake l4proxy --push

push-l7proxy-image:
	docker buildx bake l7proxy --push

push-dns-forwarder-image:
	docker buildx bake dns-forwarder --push

.PHONY: format lint clean docker-mcp-cross push-module-image mcp-package test docker-mcp push-mcp-gateway push-l4proxy-image push-l7proxy-image push-dns-forwarder-image
