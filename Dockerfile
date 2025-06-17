#syntax=docker/dockerfile:1

ARG GO_VERSION=latest
ARG GOLANGCI_LINT_VERSION=latest

FROM --platform=${BUILDPLATFORM} golangci/golangci-lint:${GOLANGCI_LINT_VERSION}-alpine AS lint-base

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION}-alpine AS base
WORKDIR /app
COPY go.* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

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

FROM base AS do-format
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install golang.org/x/tools/cmd/goimports@latest \
    && go install mvdan.cc/gofumpt@latest
COPY . .
RUN goimports -local github.com/docker/docker-mcp -w .
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


# Build the agents_gateway image
FROM golang:1.24.4-alpine3.22@sha256:68932fa6d4d4059845c8f40ad7e654e626f3ebd3706eef7846f319293ab5cb7a AS build_agents_gateway
WORKDIR /app
RUN --mount=type=cache,target=/root/.cache/go-build,id=agents_gateway \
    --mount=source=.,target=. \
    go build -o / ./cmd/docker-mcp/

FROM alpine:3.22@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715 AS agents_gateway
RUN apk add --no-cache docker-cli
ENV DOCKER_MCP_IN_CONTAINER=1
ENTRYPOINT ["/docker-mcp", "gateway", "run"]
COPY --from=build_agents_gateway /docker-mcp /