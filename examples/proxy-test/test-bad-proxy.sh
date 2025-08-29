#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== MCP Negative Test (BAD proxy) ===${NC}"

# Set proxy to a known bad address that will fail
BAD_PROXY_HOST="127.0.0.1"
BAD_PROXY_PORT="9999"  # Port that should not be listening

echo -e "${YELLOW}Using BAD proxy: ${BAD_PROXY_HOST}:${BAD_PROXY_PORT}${NC}"
echo -e "${YELLOW}This should FAIL, proving proxy variables are respected${NC}"

# Bad proxy environment variables (will be used inline, not exported)
BAD_PROXY_VARS="HTTP_PROXY=http://${BAD_PROXY_HOST}:${BAD_PROXY_PORT} HTTPS_PROXY=http://${BAD_PROXY_HOST}:${BAD_PROXY_PORT} http_proxy=http://${BAD_PROXY_HOST}:${BAD_PROXY_PORT} https_proxy=http://${BAD_PROXY_HOST}:${BAD_PROXY_PORT}"

echo -e "${YELLOW}Environment (inline only):${NC}"
echo "  HTTP_PROXY=http://${BAD_PROXY_HOST}:${BAD_PROXY_PORT}"
echo "  HTTPS_PROXY=http://${BAD_PROXY_HOST}:${BAD_PROXY_PORT}"
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

echo -e "${YELLOW}Testing MCP catalog commands with BAD proxy (should fail)...${NC}"

# Clear any existing catalog cache to force network fetch
echo -e "${YELLOW}Clearing catalog cache...${NC}"
rm -rf ~/.docker/mcp/catalogs/ ~/.docker/mcp/catalog.json 2>/dev/null || true

# Test: Show docker-mcp catalog with bad proxy (this SHOULD fail)
echo -e "${YELLOW}Test: docker mcp catalog show docker-mcp (with bad proxy)${NC}"
if timeout 15 env $BAD_PROXY_VARS $MCP_BINARY catalog show docker-mcp --format=json > /dev/null 2>&1; then
    echo -e "${RED}✗ UNEXPECTED: Catalog show succeeded with bad proxy${NC}"
    echo -e "${RED}This suggests proxy environment variables are being ignored!${NC}"
    exit 1
else
    echo -e "${GREEN}✓ EXPECTED: Catalog show failed with bad proxy${NC}"
    echo -e "${GREEN}This proves proxy environment variables are being respected${NC}"
fi

echo -e "${GREEN}=== Negative test passed! ===${NC}"
echo -e "${YELLOW}The command correctly failed when given a bad proxy,${NC}"
echo -e "${YELLOW}proving that proxy environment variables are being used.${NC}"