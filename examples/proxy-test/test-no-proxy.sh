#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== MCP Test (WITHOUT proxy) ===${NC}"

# Explicitly unset proxy environment variables
unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy NO_PROXY no_proxy

echo -e "${YELLOW}Environment: No proxy variables set${NC}"
echo ""

# Build the MCP binary to ensure we have the latest version
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

echo -e "${YELLOW}Testing MCP catalog commands with direct connection...${NC}"

# Clear any existing catalog cache to force network fetch
echo -e "${YELLOW}Clearing catalog cache...${NC}"
rm -rf ~/.docker/mcp/catalogs/ ~/.docker/mcp/catalog.json 2>/dev/null || true

# Test: Show docker-mcp catalog (this will trigger download and test network fetch)
echo -e "${YELLOW}Testing: docker mcp catalog show docker-mcp${NC}"
if CATALOG_OUTPUT=$(timeout 30 $MCP_BINARY catalog show docker-mcp --format=json 2>&1); then
    echo -e "${GREEN}✓ Catalog download successful (direct connection)${NC}"
    # Verify we actually got catalog content (not just empty response)
    if echo "$CATALOG_OUTPUT" | grep -q '"registry"'; then
        echo -e "${GREEN}✓ Catalog contains server registry${NC}"
    else
        echo -e "${RED}✗ Catalog show returned but no servers found${NC}"
        echo "Catalog output (first 200 chars): $(echo "$CATALOG_OUTPUT" | head -c 200)..."
        exit 1
    fi
else
    echo -e "${RED}✗ Catalog download failed${NC}"
    echo "Error output (first 200 chars): $(echo "$CATALOG_OUTPUT" | head -c 200)..."
    echo "This indicates a network connectivity issue"
    exit 1
fi

echo -e "${GREEN}=== All direct connection tests passed! ===${NC}"
echo -e "${YELLOW}This confirms catalog download works without proxy.${NC}"
echo -e "${YELLOW}Compare with proxy test results to verify proxy support.${NC}"