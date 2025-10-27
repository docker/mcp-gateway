#syntax=docker/dockerfile:1

ARG GO_VERSION=1.24.5

FROM --platform=${BUILDPLATFORM} golangci/golangci-lint:v2.1.6-alpine AS lint-base

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION}-alpine AS base
RUN apk add --no-cache git rsync
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
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} golangci-lint --timeout 30m0s run ./...
EOD

FROM base AS test
ARG TARGETOS
ARG TARGETARCH
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
RUN goimports -local github.com/docker/mcp-gateway-oauth-helpers -w .
RUN gofumpt -w .

FROM scratch AS format
COPY --from=do-format /app .