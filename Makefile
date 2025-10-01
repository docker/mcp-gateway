MODULE := $(shell sh -c "awk '/^module/ { print \$$2 }' go.mod")
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

export GO_LDFLAGS
DOCKER_BUILD_ARGS := --build-arg GO_LDFLAGS --build-arg DOCKER_MCP_PLUGIN_BINARY

format:
	docker buildx build $(DOCKER_BUILD_ARGS) --target=format -o . .

lint:
	docker buildx build $(DOCKER_BUILD_ARGS) --target=lint --platform=linux,darwin,windows .

lint-%:
	docker buildx build $(DOCKER_BUILD_ARGS) --target=lint --platform=$* .

clean:
	@sh -c "rm -rf bin dist"
	@sh -c "rm $(DOCKER_MCP_CLI_PLUGIN_DST)"

docker-mcp-cross:
	docker buildx build $(DOCKER_BUILD_ARGS) --target=package-docker-mcp --platform=linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64 -o ./dist .

docker-mcp-%:
	docker buildx build $(DOCKER_BUILD_ARGS) --target=package-docker-mcp --platform=$*/amd64,$*/arm64 -o ./dist .

docs:
	$(eval $@_TMP_OUT := $(shell mktemp -d -t mcp-cli-output.XXXXXXXXXX))
	docker buildx bake --set "*.output=type=local,dest=$($@_TMP_OUT)" update-docs
	rm -rf ./docs/generator/reference/*
	cp -R "$($@_TMP_OUT)"/* ./docs/generator/reference/
	rm -rf "$($@_TMP_OUT)"/*

push-module-image:
	cp -r dist ./module-image
	docker buildx build --push --platform=linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64 --build-arg TAG=$(TAG) --tag=docker/docker-mcp-cli-desktop-module:$(TAG) ./module-image
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

integration:
	go test -count=1 ./... -run 'TestIntegration'

docker-mcp:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w ${GO_LDFLAGS}" -o ./dist/$(DOCKER_MCP_PLUGIN_BINARY)$(EXTENSION) ./cmd/docker-mcp
	rm "$(DOCKER_MCP_CLI_PLUGIN_DST)" || true
	cp "dist/$(DOCKER_MCP_PLUGIN_BINARY)$(EXTENSION)" "$(DOCKER_MCP_CLI_PLUGIN_DST)"

push-mcp-gateway:
	docker buildx bake mcp-gateway mcp-gateway-dind --push

push-l4proxy-image:
	docker buildx bake l4proxy --push

push-l7proxy-image:
	docker buildx bake l7proxy --push

push-dns-forwarder-image:
	docker buildx bake dns-forwarder --push

.PHONY: format lint clean docker-mcp-cross push-module-image mcp-package test docker-mcp push-mcp-gateway push-l4proxy-image push-l7proxy-image push-dns-forwarder-image docs
