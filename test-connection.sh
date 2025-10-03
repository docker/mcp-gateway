#!/bin/bash
# Test script for MCP Gateway connection

echo "Testing connection to MCP Gateway..."

# Test if the gateway is reachable
if nc -z 192.168.1.231 8080; then
    echo "✅ MCP Gateway is reachable on 192.168.1.231:8080"
else
    echo "❌ Cannot reach MCP Gateway on 192.168.1.231:8080"
    echo "Make sure the gateway is running with:"
    echo "docker compose -f docker-compose-remote.yaml up -d"
    exit 1
fi

# Test the Python bridge script
echo "Testing Python bridge script..."
if python3 /Users/bits/mcp-gateway/python_tools/remote_claude_mcp_configs.py --help > /dev/null 2>&1; then
    echo "✅ Python bridge script is working"
else
    echo "❌ Python bridge script has issues"
    exit 1
fi

echo ""
echo "✅ Everything looks good!"
echo ""
echo "Next steps:"
echo "1. On remote machine (192.168.1.231):"
echo "   docker compose -f docker-compose-remote.yaml up -d"
echo ""
echo "2. Update your Claude Desktop config with:"
echo "   /Users/bits/mcp-gateway/claude-desktop-config.json"
echo ""
echo "3. Restart Claude Desktop"