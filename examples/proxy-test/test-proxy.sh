#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== MCP Proxy Test (WITH proxy) ===${NC}"

# Check if proxy is running
if ! docker compose ps --services --filter "status=running" | grep -q "proxy"; then
    echo -e "${RED}Error: Proxy service not running. Start with: docker compose up -d${NC}"
    exit 1
fi

# Get proxy container IP
PROXY_HOST="localhost"
PROXY_PORT="3128"

echo -e "${YELLOW}Using proxy: ${PROXY_HOST}:${PROXY_PORT}${NC}"

# Proxy environment variables (will be used inline, not exported)
PROXY_VARS="HTTP_PROXY=http://${PROXY_HOST}:${PROXY_PORT} HTTPS_PROXY=http://${PROXY_HOST}:${PROXY_PORT} http_proxy=http://${PROXY_HOST}:${PROXY_PORT} https_proxy=http://${PROXY_HOST}:${PROXY_PORT}"

echo -e "${YELLOW}Environment (inline only):${NC}"
echo "  HTTP_PROXY=http://${PROXY_HOST}:${PROXY_PORT}"
echo "  HTTPS_PROXY=http://${PROXY_HOST}:${PROXY_PORT}"
echo ""

# Build the MCP binary to ensure we have the latest version with the fix
echo -e "${YELLOW}Building docker-mcp binary...${NC}"
PROJECT_ROOT="$(cd ../../ && pwd)"
cd "$PROJECT_ROOT"
make docker-mcp || {
    echo -e "${RED}Failed to build docker-mcp binary${NC}"
    exit 1
}

# Use the built binary with absolute path
MCP_BINARY="$PROJECT_ROOT/dist/docker-mcp"
cd "$PROJECT_ROOT/examples/proxy-test/"

# Verify binary exists
if [[ ! -f "$MCP_BINARY" ]]; then
    echo -e "${RED}Error: docker-mcp binary not found at $MCP_BINARY${NC}"
    echo "Run 'make docker-mcp' from the project root first"
    exit 1
fi

echo -e "${YELLOW}Testing MCP catalog commands through proxy...${NC}"

# Clear any existing catalog cache to force network fetch
echo -e "${YELLOW}Clearing catalog cache...${NC}"
rm -rf ~/.docker/mcp/catalogs/ ~/.docker/mcp/catalog.json 2>/dev/null || true

# Test 1: Show docker-mcp catalog (this will trigger download through proxy)
echo -e "${YELLOW}Test 1: docker mcp catalog show docker-mcp${NC}"
if CATALOG_OUTPUT=$(timeout 30 env $PROXY_VARS $MCP_BINARY catalog show docker-mcp --format=json 2>&1); then
    echo -e "${GREEN}✓ Catalog download successful through proxy${NC}"
    # Verify we actually got catalog content (not just empty response)
    if echo "$CATALOG_OUTPUT" | grep -q '"registry"'; then
        echo -e "${GREEN}✓ Catalog contains server registry${NC}"
    else
        echo -e "${RED}✗ Catalog show returned but no servers found${NC}"
        echo "Catalog output (first 200 chars): $(echo "$CATALOG_OUTPUT" | head -c 200)..."
        exit 1
    fi
else
    echo -e "${RED}✗ Catalog download failed through proxy${NC}"
    echo "Error output (first 200 chars): $(echo "$CATALOG_OUTPUT" | head -c 200)..."
    echo "This indicates the proxy fix didn't work"
    exit 1
fi

# Test 2: Force catalog update to test proxy with different command
echo -e "${YELLOW}Test 2: docker mcp catalog update (force network fetch)${NC}"
if timeout 30 env $PROXY_VARS $MCP_BINARY catalog update 2>/dev/null; then
    echo -e "${GREEN}✓ Catalog update successful through proxy${NC}"
else
    echo -e "${RED}✗ Catalog update failed${NC}"
    echo "This might indicate the proxy fix didn't work for update command"
    exit 1
fi

echo -e "${GREEN}=== All proxy tests passed! ===${NC}"
echo -e "${YELLOW}Check proxy logs to see the HTTP requests:${NC}"
echo "  docker compose logs proxy | grep desktop.docker.com"
echo ""
echo -e "${YELLOW}To see recent proxy activity:${NC}"
echo "  docker compose logs --tail=20 proxy"