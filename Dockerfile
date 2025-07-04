#syntax=docker/dockerfile:1

ARG GO_VERSION=latest
ARG GOLANGCI_LINT_VERSION=latest

FROM --platform=${BUILDPLATFORM} golangci/golangci-lint:${GOLANGCI_LINT_VERSION}-alpine AS lint-base

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION}-alpine AS base
RUN apk add --no-cache git
WORKDIR /app

FROM base AS lint
COPY --from=lint-base /usr/bin/golangci-lint /usr/bin/golangci-lint
ARG TARGETOS
ARG TARGETARCH
RUN --mount=target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/.cache/golangci-lint <<EOD
    set -e
    go mod tidy --diff
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} golangci-lint --timeout 30m0s run ./...
EOD

FROM base AS test
RUN --mount=target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build <<EOD
    set -e
    CGO_ENABLED=0 go test -short --count=1 -v ./...
EOD

FROM base AS do-format
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install golang.org/x/tools/cmd/goimports@latest \
    && go install mvdan.cc/gofumpt@latest
COPY . .
RUN rm -rf vendor
RUN goimports -local github.com/docker/mcp-gateway -w .
RUN gofumpt -w .

FROM scratch AS format
COPY --from=do-format /app .

FROM base AS build-docker-mcp
ARG TARGETOS
ARG TARGETARCH
ARG GO_LDFLAGS
ARG DOCKER_MCP_PLUGIN_BINARY
RUN --mount=target=.\
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags "-s -w ${GO_LDFLAGS}" -o /out/${DOCKER_MCP_PLUGIN_BINARY} ./cmd/docker-mcp

FROM scratch AS binary-docker-mcp-unix
ARG DOCKER_MCP_PLUGIN_BINARY
COPY --link --from=build-docker-mcp /out/${DOCKER_MCP_PLUGIN_BINARY} /

FROM binary-docker-mcp-unix AS binary-docker-mcp-darwin

FROM binary-docker-mcp-unix AS binary-docker-mcp-linux

FROM scratch AS binary-docker-mcp-windows
ARG DOCKER_MCP_PLUGIN_BINARY
COPY --link --from=build-docker-mcp /out/${DOCKER_MCP_PLUGIN_BINARY} /${DOCKER_MCP_PLUGIN_BINARY}.exe

FROM binary-docker-mcp-$TARGETOS AS binary-docker-mcp

FROM --platform=$BUILDPLATFORM alpine AS packager-docker-mcp
WORKDIR /mcp
ARG DOCKER_MCP_PLUGIN_BINARY
RUN --mount=from=binary-docker-mcp mkdir -p /out && cp ${DOCKER_MCP_PLUGIN_BINARY}* /out/
FROM scratch AS package-docker-mcp
COPY --from=packager-docker-mcp /out .


# Build the mcp-gateway image
FROM golang:1.24.4-alpine3.22@sha256:68932fa6d4d4059845c8f40ad7e654e626f3ebd3706eef7846f319293ab5cb7a AS build-mcp-gateway
WORKDIR /app
RUN --mount=type=cache,target=/root/.cache/go-build,id=mcp-gateway \
    --mount=source=.,target=. \
    go build -trimpath -ldflags "-s -w" -o / ./cmd/docker-mcp/

FROM golang:1.24.4-alpine3.22@sha256:68932fa6d4d4059845c8f40ad7e654e626f3ebd3706eef7846f319293ab5cb7a AS build-mcp-bridge
WORKDIR /app
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=source=./tools/docker-mcp-bridge,target=. \
    go build -trimpath -ldflags "-s -w" -o /docker-mcp-bridge .

FROM alpine:3.22@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715 AS mcp-gateway
RUN apk add --no-cache docker-cli socat jq
VOLUME /misc
COPY --from=build-mcp-bridge /docker-mcp-bridge /misc/
ENV DOCKER_MCP_IN_CONTAINER=1
ENTRYPOINT ["/docker-mcp", "gateway", "run"]
COPY --from=build-mcp-gateway /docker-mcp /

FROM docker:dind@sha256:0a2ee60851e1b61a54707476526c4ed48cc55641a17a5cba8a77fb78e7a4742c AS dind
RUN rm /usr/local/bin/docker-compose \
    /usr/local/libexec/docker/cli-plugins/docker-compose \
    /usr/local/libexec/docker/cli-plugins/docker-buildx

FROM scratch AS mcp-gateway-dind
COPY --from=dind / /
RUN apk add --no-cache socat jq
COPY --from=docker/mcp-gateway /docker-mcp /
RUN cat <<-'EOF' >/run.sh
	#!/usr/bin/env sh
	set -euxo pipefail

	echo "Starting dockerd..."
	export TINI_SUBREAPER=1
	export DOCKER_DRIVER=vfs
	dockerd-entrypoint.sh dockerd &

	until docker info > /dev/null 2>&1
	do
	echo "Waiting for dockerd..."
	sleep 1
	done
	echo "Detected dockerd ready for work!"

	export DOCKER_MCP_IN_CONTAINER=1
	export DOCKER_MCP_IN_DIND=1
	echo "Starting MCP Gateway on port $PORT..."
	exec /docker-mcp gateway run --port=$PORT "$@"
EOF
RUN chmod +x /run.sh
ENV PORT=8080
ENTRYPOINT ["/run.sh"]
