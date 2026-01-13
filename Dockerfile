#syntax=docker/dockerfile:1

ARG GO_VERSION=1.25.5
ARG DOCS_FORMATS="md,yaml"

FROM --platform=${BUILDPLATFORM} golangci/golangci-lint:v2.8.0-alpine AS lint-base

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION}-alpine AS base
RUN apk add --no-cache git rsync
WORKDIR /app

# Docs generation and validation targets
FROM base AS docs-gen
WORKDIR /src
RUN --mount=target=. \
    --mount=target=/root/.cache,type=cache \
    go build -mod=vendor -o /out/docsgen ./docs/generator/generate.go

FROM base AS docs-build
COPY --from=docs-gen /out/docsgen /usr/bin
ENV DOCKER_CLI_PLUGIN_ORIGINAL_CLI_COMMAND="mcp"
ARG DOCS_FORMATS
RUN --mount=target=/context \
    --mount=target=.,type=tmpfs <<EOT
  set -e
  rsync -a /context/. .
  docsgen --formats "$DOCS_FORMATS" --source "docs/generator/reference"
  mkdir /out
  cp -r docs/generator/reference/* /out/
EOT

FROM scratch AS docs-update
COPY --from=docs-build /out /

FROM docs-build AS docs-validate
RUN --mount=target=/context \
    --mount=target=.,type=tmpfs <<EOT
  set -e
  rsync -a /context/. .
  git add -A
  rm -rf docs/generator/reference/*
  cp -rf /out/* ./docs/generator/reference/
  if [ -n "$(git status --porcelain -- docs/generator/reference)" ]; then
    echo >&2 'ERROR: Docs result differs. Rebase on main branch and rerun "make docs"'
    git status --porcelain -- docs/generator/reference
    exit 1
  fi
EOT

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
FROM golang:${GO_VERSION}-alpine AS build-mcp-gateway
WORKDIR /app
RUN --mount=type=cache,target=/root/.cache/go-build,id=mcp-gateway \
    --mount=source=.,target=. \
    go build -trimpath -ldflags "-s -w" -o / ./cmd/docker-mcp/

FROM golang:${GO_VERSION}-alpine AS build-mcp-bridge
WORKDIR /app
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=source=./tools/docker-mcp-bridge,target=. \
    go build -trimpath -ldflags "-s -w" -o /docker-mcp-bridge .

FROM alpine:3.23@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62 AS mcp-gateway
RUN apk update && apk upgrade --no-cache && apk add --no-cache docker-cli socat jq
VOLUME /misc
COPY --from=build-mcp-bridge /docker-mcp-bridge /misc/
ENV DOCKER_MCP_IN_CONTAINER=1
ENTRYPOINT ["/docker-mcp", "gateway", "run"]
COPY --from=build-mcp-gateway /docker-mcp /

FROM docker:dind@sha256:4dd2f7e405b1a10fda628f22cd466be1e3be2bcfc46db653ab620e02eeed5794 AS dind
RUN rm /usr/local/bin/docker-compose \
    /usr/local/libexec/docker/cli-plugins/docker-compose \
    /usr/local/libexec/docker/cli-plugins/docker-buildx

FROM scratch AS mcp-gateway-dind
COPY --from=dind / /
RUN apk update && apk upgrade --no-cache && apk add --no-cache socat jq
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
