#!/usr/bin/env sh
set -euxo pipefail

echo "Starting dockerd..."
export TINI_SUBREAPER=1
dockerd-entrypoint.sh dockerd &

until docker info > /dev/null 2>&1
do
  echo "Waiting for dockerd..."
  sleep 1
done
echo "Detected dockerd ready for work!"

echo "Starting MCP Gateway on port $PORT..."
export DOCKER_MCP_IN_CONTAINER=1
export DOCKER_MCP_IN_DIND=1
exec /docker-mcp gateway run "$@"
