# Standalone MCP Gateway with UI

A standalone Model Context Protocol (MCP) gateway with a web-based user interface for dynamic server management.

## Features

🎛️ **Web-based UI**: Modern, responsive interface for managing MCP servers
🔄 **Dynamic Server Management**: Add, remove, and configure MCP servers on the fly
📋 **Configuration Export**: Generate configs for Claude Desktop, LLM Studio, and Docker Compose
🐳 **Docker Integration**: Full Docker MCP catalog support
🔧 **Real-time Updates**: Live server status and configuration changes
🌐 **Remote Access**: Support for remote gateway connections

## Quick Start

### Option 1: Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/davesbits/mcp-gateway.git
cd mcp-gateway

# Start the standalone gateway
docker compose -f compose.standalone.yaml up -d

# Access the UI
open http://localhost:3000
```

### Option 2: Build from Source

```bash
# Prerequisites: Go 1.24+, Docker

# Clone and build
git clone https://github.com/davesbits/mcp-gateway.git
cd mcp-gateway
go build -o standalone-gateway ./cmd/standalone-gateway

# Run
./standalone-gateway

# Access the UI
open http://localhost:3000
```

## Usage

### Web Interface

1. **Server Management**: View, add, and remove MCP servers from the catalog
2. **Configuration**: Set up gateway transport, ports, and features
3. **Export Configs**: Generate configuration files for MCP clients
4. **Remote Connection**: Connect to remote MCP gateways

### API Endpoints

- `GET /api/servers` - List all available servers
- `POST /api/servers/add` - Add a server to the gateway
- `POST /api/servers/remove` - Remove a server from the gateway
- `GET /api/config` - Get gateway configuration
- `POST /api/config` - Update gateway configuration
- `GET /api/export/claude` - Export Claude Desktop configuration
- `GET /api/export/llmstudio` - Export LLM Studio configuration
- `GET /api/export/docker-compose` - Export Docker Compose configuration

### MCP Client Configuration

#### Claude Desktop

1. Go to the "Export Config" tab in the UI
2. Click "Claude Desktop" to generate the configuration
3. Copy the JSON to your Claude Desktop config file (`~/.claude_desktop_config.json`)

Example configuration:
```json
{
  "mcpServers": {
    "MCP_GATEWAY": {
      "command": "docker",
      "args": ["mcp", "gateway", "run", "--servers=filesystem,duckduckgo"],
      "env": {}
    }
  }
}
```

#### LLM Studio

1. Generate the LLM Studio configuration from the UI
2. Use the SSE endpoint: `http://localhost:8811/sse`

#### Remote Access

Connect to the gateway remotely:
```bash
# From another machine
curl http://your-server:8811/sse
```

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web UI        │    │  Standalone     │    │  MCP Gateway    │
│   (Port 3000)   │◄───┤  Gateway        │◄───┤  (Port 8811)    │
│                 │    │  HTTP Server    │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                       │
                                │                       ▼
                                │               ┌─────────────────┐
                                │               │  Docker         │
                                │               │  MCP Servers    │
                                │               └─────────────────┘
                                ▼
                        ┌─────────────────┐
                        │  Configuration  │
                        │  Management     │
                        └─────────────────┘
```

## Configuration

### Environment Variables

- `PORT` - UI server port (default: 3000)
- `MCP_GATEWAY_PORT` - MCP gateway port (default: 8811)
- `MCP_ENABLE_DYNAMIC_TOOLS` - Enable dynamic server management (default: true)

### Default Configuration

```yaml
gateway:
  port: 8811
  transport: sse
  catalogUrl: "https://desktop.docker.com/mcp/catalog/v2/catalog.yaml"
  enableDynamicTools: true
  enableLogging: true
  defaultServers:
    - filesystem
    - duckduckgo
```

## Development

### Building

```bash
# Build the standalone gateway
go build -o standalone-gateway ./cmd/standalone-gateway

# Build Docker image
docker build -f Dockerfile.standalone -t mcp-gateway-standalone .
```

### Project Structure

```
cmd/standalone-gateway/
├── main.go                 # Standalone gateway server
ui/
├── index.html             # Web interface
├── app.js                 # JavaScript application
compose.standalone.yaml    # Docker Compose setup
Dockerfile.standalone      # Docker build file
```

## Features in Detail

### Dynamic Server Management

- **Real-time Addition/Removal**: Add or remove MCP servers without restarting
- **Catalog Integration**: Browse and search the complete Docker MCP catalog
- **Configuration Management**: Set server-specific configurations
- **Status Monitoring**: View active/inactive server status

### Configuration Export

Generate configuration files for:
- **Claude Desktop**: STDIO and SSE configurations
- **LLM Studio**: Remote SSE endpoint configurations  
- **Docker Compose**: Full deployment setups

### Remote Operations

- **Remote Gateway Connection**: Connect to MCP gateways on other machines
- **Multi-gateway Support**: Manage multiple gateway instances
- **Cross-platform Access**: Web-based interface accessible from any device

## Troubleshooting

### Common Issues

1. **Gateway won't start**: Ensure Docker is running and accessible
2. **UI not accessible**: Check firewall settings for port 3000
3. **Servers won't add**: Verify Docker socket access (`/var/run/docker.sock`)

### Logs

```bash
# Docker Compose logs
docker compose -f compose.standalone.yaml logs -f

# Direct execution logs
./standalone-gateway 2>&1 | tee gateway.log
```

### Health Checks

- UI Health: `curl http://localhost:3000`
- MCP Gateway Health: `curl http://localhost:8811/health`
- API Health: `curl http://localhost:3000/api/servers`

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes and test
4. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](../../LICENSE) file for details.