#!/bin/bash
# Test script for MCP Gateway OpenTelemetry integration
# This script helps verify that telemetry is properly flowing from docker-mcp to collectors

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo "=== MCP Gateway Telemetry Test Suite ==="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check prerequisites
check_prerequisites() {
    echo "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}❌ Docker is not installed${NC}"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        echo -e "${YELLOW}⚠️  jq is not installed (optional but recommended)${NC}"
    fi
    
    echo -e "${GREEN}✅ Prerequisites met${NC}"
    echo ""
}

# Show current OTEL configuration
show_config() {
    echo "Current OTEL Configuration:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    echo "1. Environment Variables:"
    echo "   OTEL_EXPORTER_OTLP_ENDPOINT: ${OTEL_EXPORTER_OTLP_ENDPOINT:-not set}"
    echo "   DOCKER_CLI_OTEL_EXPORTER_OTLP_ENDPOINT: ${DOCKER_CLI_OTEL_EXPORTER_OTLP_ENDPOINT:-not set}"
    echo "   DOCKER_MCP_TELEMETRY_DEBUG: ${DOCKER_MCP_TELEMETRY_DEBUG:-not set}"
    echo ""
    
    echo "2. Docker Context OTEL Configuration:"
    if command -v jq &> /dev/null; then
        docker context inspect 2>/dev/null | jq '.[].Metadata.otel' 2>/dev/null || echo "   No OTEL config in Docker context"
    else
        docker context inspect 2>/dev/null | grep -A5 '"otel"' || echo "   No OTEL config in Docker context"
    fi
    echo ""
}

# Start OTEL collector
start_collector() {
    echo "Starting OpenTelemetry Collector..."
    
    # Check if already running
    if docker ps | grep -q otel-collector-debug; then
        echo -e "${YELLOW}⚠️  Collector already running, stopping it first${NC}"
        docker stop otel-collector-debug 2>/dev/null || true
        sleep 2
    fi
    
    # Ask which config to use (with timeout for non-interactive mode)
    if [ -t 0 ]; then
        echo "Select collector configuration:"
        echo "1) Debug output only (default)"
        echo "2) Debug + Prometheus export (port 8889)"
        read -t 5 -p "Choice [1]: " config_choice
        config_choice=${config_choice:-1}
    else
        config_choice=1
    fi
    
    if [ "$config_choice" = "2" ]; then
        CONFIG_FILE="otel-collector-prometheus.yaml"
        EXTRA_PORTS="-p 8889:8889"
        echo "Using Prometheus configuration..."
    else
        CONFIG_FILE="otel-collector-config.yaml"
        EXTRA_PORTS=""
        echo "Using debug-only configuration..."
    fi
    
    # Start collector
    docker run --rm -d \
        --name otel-collector-debug \
        -p 4317:4317 \
        -p 4318:4318 \
        ${EXTRA_PORTS} \
        -v "${SCRIPT_DIR}/${CONFIG_FILE}:/etc/otel-collector-config.yaml" \
        otel/opentelemetry-collector:latest \
        --config=/etc/otel-collector-config.yaml
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ Collector started on localhost:4317 (gRPC) and localhost:4318 (HTTP)${NC}"
        if [ "$config_choice" = "2" ]; then
            echo -e "${GREEN}   Prometheus metrics available at http://localhost:8889/metrics${NC}"
        fi
    else
        echo -e "${RED}❌ Failed to start collector${NC}"
        exit 1
    fi
    
    # Give it a moment to start
    sleep 2
    echo ""
}

# Test telemetry flow
test_telemetry() {
    echo "Testing Telemetry Flow..."
    echo "━━━━━━━━━━━━━━━━━━━━━━"
    
    # Create a test catalog if needed
    if [ ! -f ~/.docker/mcp/catalogs/docker-mcp.yaml ]; then
        echo -e "${YELLOW}⚠️  No catalog found, using test catalog${NC}"
        cat > /tmp/test-catalog.yaml <<EOF
name: test-catalog
displayName: Test Catalog
registry:
  dockerhub:
    image: mcp/dockerhub@sha256:b3a124cc092a2eb24b3cad69d9ea0f157581762d993d599533b1802740b2c262
    command:
      - --transport=stdio
EOF
        CATALOG_PATH="/tmp/test-catalog.yaml"
    else
        CATALOG_PATH="~/.docker/mcp/catalogs/docker-mcp.yaml"
    fi
    
    echo "1. Running docker-mcp with debug logging..."
    export DOCKER_MCP_TELEMETRY_DEBUG=1
    export DOCKER_CLI_OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
    
    # Run a simple command that should generate telemetry
    timeout 5 docker mcp gateway run --catalog "$CATALOG_PATH" --dry-run 2>&1 | grep -E "MCP-TELEMETRY|Initialized" || true
    
    echo ""
    echo "2. Checking collector for received telemetry..."
    sleep 2
    
    # Check collector logs
    if docker logs otel-collector-debug 2>&1 | grep -q "InstrumentationScope github.com/docker/mcp-gateway"; then
        echo -e "${GREEN}✅ MCP Gateway telemetry detected!${NC}"
        echo ""
        echo "Sample telemetry data:"
        docker logs otel-collector-debug 2>&1 | grep -A10 "github.com/docker/mcp-gateway" | head -20
    else
        echo -e "${YELLOW}⚠️  No MCP Gateway telemetry detected yet${NC}"
        echo "This is normal if no tool calls were made."
        echo ""
        echo "Docker CLI telemetry:"
        docker logs otel-collector-debug 2>&1 | grep -A5 "github.com/docker/cli" | head -15 || echo "No Docker CLI telemetry found"
    fi
    echo ""
}

# View collector logs
view_logs() {
    echo "Collector Logs (last 50 lines):"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    docker logs --tail 50 otel-collector-debug 2>&1
}

# Cleanup
cleanup() {
    echo ""
    echo "Cleaning up..."
    docker stop otel-collector-debug 2>/dev/null || true
    echo -e "${GREEN}✅ Cleanup complete${NC}"
}

# Interactive test with tool calls
interactive_test() {
    echo ""
    echo "Interactive Testing Mode"
    echo "━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "To test with actual tool calls:"
    echo ""
    echo "1. Start the gateway in SSE mode:"
    echo -e "${YELLOW}   export DOCKER_MCP_TELEMETRY_DEBUG=1${NC}"
    echo -e "${YELLOW}   export DOCKER_CLI_OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317${NC}"
    echo -e "${YELLOW}   docker mcp gateway run --transport sse --port 3000${NC}"
    echo ""
    echo "2. Make tool calls to: http://localhost:3000/sse"
    echo ""
    echo "3. Watch collector logs in another terminal:"
    echo -e "${YELLOW}   docker logs -f otel-collector-debug | grep mcp${NC}"
    echo ""
}

# Main menu
main_menu() {
    PS3='Please select an option: '
    options=(
        "Run full test suite"
        "Start collector only"
        "View collector logs"
        "Show interactive test instructions"
        "Cleanup and exit"
    )
    
    select opt in "${options[@]}"
    do
        case $opt in
            "Run full test suite")
                check_prerequisites
                show_config
                start_collector
                test_telemetry
                cleanup
                break
                ;;
            "Start collector only")
                check_prerequisites
                start_collector
                echo "Collector running. Use 'docker logs -f otel-collector-debug' to view output"
                break
                ;;
            "View collector logs")
                view_logs
                break
                ;;
            "Show interactive test instructions")
                interactive_test
                break
                ;;
            "Cleanup and exit")
                cleanup
                break
                ;;
            *) echo "Invalid option $REPLY";;
        esac
    done
}

# Handle Ctrl+C
trap cleanup INT

# Run based on arguments
if [ "$1" == "--help" ] || [ "$1" == "-h" ]; then
    echo "Usage: $0 [option]"
    echo ""
    echo "Options:"
    echo "  --full        Run full test suite"
    echo "  --start       Start collector only (debug mode)"
    echo "  --start-prom  Start collector with Prometheus export"
    echo "  --logs        View collector logs"
    echo "  --cleanup     Stop and remove collector"
    echo "  --help        Show this help"
    echo ""
    echo "Without options, shows interactive menu"
elif [ "$1" == "--full" ]; then
    check_prerequisites
    show_config
    start_collector
    test_telemetry
    cleanup
elif [ "$1" == "--start" ]; then
    check_prerequisites
    config_choice=1
    start_collector
elif [ "$1" == "--start-prom" ]; then
    check_prerequisites
    config_choice=2
    start_collector
elif [ "$1" == "--logs" ]; then
    view_logs
elif [ "$1" == "--cleanup" ]; then
    cleanup
else
    main_menu
fi