#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=====================================${NC}"
echo -e "${BLUE}  MCP Proxy Test Suite${NC}"
echo -e "${BLUE}=====================================${NC}"
echo ""

# Function to cleanup on exit
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    docker compose down --volumes --timeout 10 2>/dev/null || true
}
trap cleanup EXIT

# Start proxy server
echo -e "${YELLOW}Starting proxy server...${NC}"
docker compose up -d

# Wait for proxy to be healthy
echo -e "${YELLOW}Waiting for proxy to be ready...${NC}"
timeout 60 bash -c 'until docker compose ps --services --filter "status=running" | grep -q "proxy"; do sleep 2; done'

echo -e "${GREEN}✓ Proxy server is running${NC}"
echo ""

# Show proxy info
echo -e "${BLUE}Proxy Information:${NC}"
echo "  Container: $(docker compose ps --format 'table {{.Name}}\t{{.Status}}' | grep proxy)"
echo "  Proxy URL: http://localhost:3128"
echo ""

# Run tests without proxy (baseline)
echo -e "${BLUE}Running baseline test (no proxy)...${NC}"
./test-no-proxy.sh
echo ""

# Run negative test with bad proxy (should fail)
echo -e "${BLUE}Running negative test (bad proxy)...${NC}"
./test-bad-proxy.sh
echo ""

# Run tests with proxy
echo -e "${BLUE}Running proxy test...${NC}"  
./test-proxy.sh
echo ""

# Show proxy logs
echo -e "${BLUE}Recent proxy access logs:${NC}"
echo -e "${YELLOW}(Look for requests to desktop.docker.com)${NC}"
docker compose logs --tail=50 proxy | grep -E "(GET|CONNECT|desktop\.docker\.com)" || echo "No matching requests found in recent logs"
echo ""

echo -e "${GREEN}=====================================${NC}"
echo -e "${GREEN}  All tests completed successfully!${NC}"  
echo -e "${GREEN}=====================================${NC}"
echo ""
echo -e "${YELLOW}Summary:${NC}"
echo "  ✓ Direct catalog download works"
echo "  ✓ Bad proxy correctly fails (proves env vars are used)"
echo "  ✓ Good proxy catalog download works" 
echo "  ✓ HTTP client properly respects proxy environment variables"
echo "  ✓ Catalog content validation ensures real network fetch"
echo ""
echo -e "${YELLOW}The proxy fix has been thoroughly verified!${NC}"