# Remote MCP Gateway Setup

This setup allows you to run MCP servers on a remote machine and connect to them from Claude Desktop or VS Code on your local machine.

## Files Created

- `python_tools/remote_claude_mcp_configs.py` - Bridge script for stdio-to-network translation
- `docker-compose-remote.yaml` - Docker Compose for remote MCP Gateway
- `claude-desktop-config.json` - Claude Desktop configuration
- `test-connection.sh` - Test script to verify setup

## Setup Instructions

### 1. On Remote Machine (192.168.1.231)

1. Copy the `docker-compose-remote.yaml` file to your remote machine
2. Start the MCP Gateway:
   ```bash
   docker compose -f docker-compose-remote.yaml up -d
   ```
3. Verify it's running:
   ```bash
   docker compose -f docker-compose-remote.yaml logs gateway
   ```

### 2. On Local Machine

1. Test the connection:
   ```bash
   ./test-connection.sh
   ```

2. Update your Claude Desktop configuration:
   - Location: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - Copy content from `claude-desktop-config.json`

3. Restart Claude Desktop

## Troubleshooting

### Common Issues

1. **HTTP/1.1 errors**: The gateway is running in wrong transport mode
   - Solution: Use `--transport=stdio` in Docker command

2. **Python syntax errors**: Using older Python version
   - Solution: Use `/usr/local/bin/python3` explicitly

3. **Connection refused**: Gateway not accessible
   - Check if gateway is running: `nc -z 192.168.1.231 8080`
   - Check firewall settings on remote machine

### Testing Individual Components

1. **Test gateway directly**:
   ```bash
   nc 192.168.1.231 8080
   ```

2. **Test Python bridge**:
   ```bash
   python3 python_tools/remote_claude_mcp_configs.py 192.168.1.231 8080
   ```

3. **Check gateway logs**:
   ```bash
   docker compose -f docker-compose-remote.yaml logs -f gateway
   ```

## Configuration Details

### Transport Protocol
- Gateway runs with `--transport=stdio` for bridge compatibility
- Bridge translates between stdio (Claude) and network socket (Gateway)

### Network Setup
- Gateway binds to `0.0.0.0:8080` for LAN access
- Bridge connects to specific IP (`192.168.1.231:8080`)

### Python Compatibility
- Script uses Python 2/3 compatible syntax
- No type hints for older Python versions
- Explicit error handling and formatting