# Standalone MCP Gateway with Web UI

A complete standalone Model Context Protocol (MCP) gateway with a modern web-based user interface for dynamic server management.

## 🎯 Overview

This standalone gateway provides a user-friendly web interface to manage MCP servers dynamically, generate configuration files for popular MCP clients, and deploy complete MCP solutions with Docker.

![MCP Gateway Manager UI](https://github.com/user-attachments/assets/9264cf08-62ae-4be1-b4ed-f3427b54341e)

## ✨ Key Features

🎛️ **Modern Web Interface**
- Responsive design with intuitive server management
- Real-time server status indicators
- Search and filter capabilities

🔄 **Dynamic Server Management**
- Add/remove servers on the fly
- Complete Docker MCP catalog integration
- Server configuration management

📋 **Configuration Export**
- Claude Desktop (`mcp.json`) generation
- LLM Studio configurations
- Docker Compose deployments

🌐 **Remote Operations**
- Remote gateway connections
- Cross-platform web access
- API-driven management

## 🚀 Quick Start

### Option 1: Docker Compose (Recommended)

```bash
# Clone and start
git clone https://github.com/davesbits/mcp-gateway.git
cd mcp-gateway
docker compose -f compose.standalone.yaml up -d

# Access the UI
open http://localhost:3000
```

### Option 2: Build from Source

```bash
# Prerequisites: Go 1.24+
git clone https://github.com/davesbits/mcp-gateway.git
cd mcp-gateway
go build -o standalone-gateway ./cmd/standalone-gateway
./standalone-gateway

# Access the UI
open http://localhost:3000
```

## 📖 Usage Guide

### Server Management

1. **View Servers**: Browse available MCP servers with status indicators
2. **Add Servers**: Click "Add" on inactive servers to enable them
3. **Remove Servers**: Click "Remove" on active servers to disable them
4. **Configure**: Modify server-specific configurations
5. **Search**: Use the search box to find specific servers

### Configuration Export

#### For Claude Desktop:
1. Go to "Export Config" tab
2. Click "Claude Desktop"
3. Copy the generated JSON to `~/.claude_desktop_config.json`

Example output:
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

#### For LLM Studio:
1. Click "LLM Studio" in Export Config
2. Use the SSE endpoint: `http://localhost:8811/sse`

#### For Docker Deployment:
1. Click "Docker Compose" 
2. Save output as `docker-compose.yml`
3. Run: `docker compose up -d`

### Remote Gateway Management

1. Go to "Settings" tab
2. Configure remote host and protocol
3. Click "Connect to Remote"
4. Manage remote MCP gateways through the UI

## 🏗️ Architecture

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

## 🔧 API Endpoints

- `GET /api/servers` - List all available servers
- `POST /api/servers/add` - Add a server to the gateway
- `POST /api/servers/remove` - Remove a server from the gateway
- `GET /api/config` - Get gateway configuration
- `POST /api/config` - Update gateway configuration
- `GET /api/export/claude` - Export Claude Desktop configuration
- `GET /api/export/llmstudio` - Export LLM Studio configuration
- `GET /api/export/docker-compose` - Export Docker Compose configuration

## 🛠️ Development

### Building

```bash
# Build standalone gateway
go build -o standalone-gateway ./cmd/standalone-gateway

# Build Docker image
docker build -f Dockerfile.standalone -t mcp-gateway-standalone .
```

### Project Structure

```
cmd/standalone-gateway/
├── main.go                 # Standalone gateway server
├── ui/                     # Embedded web UI files
│   ├── index.html         # Main interface
│   └── app.js             # JavaScript application
└── README.md              # Detailed documentation

ui/                         # Source UI files
├── index.html             # Web interface
└── app.js                 # Application logic

compose.standalone.yaml     # Docker Compose setup
Dockerfile.standalone       # Docker build file
```

## 🌟 Sample Servers

The UI comes pre-configured with sample MCP servers:

- **filesystem**: File operations (read, write, list, create)
- **duckduckgo**: Web search capabilities
- **github**: Repository management (requires GITHUB_TOKEN)
- **postgres**: Database operations (requires connection string)
- **slack**: Messaging integration (requires SLACK_BOT_TOKEN)

## 🔒 Security

- CORS-enabled for cross-origin requests
- Docker socket access for container management
- Environment-based secret management
- Production-ready containerization

## 📚 Related Documentation

- [MCP Gateway Documentation](docs/mcp-gateway.md)
- [Dynamic Server Management](docs/feature-specs/dynamic_servers.md)
- [Docker MCP Catalog](https://hub.docker.com/mcp)
- [Model Context Protocol Specification](https://spec.modelcontextprotocol.io/)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Implement changes with tests
4. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

Built with ❤️ for the MCP community