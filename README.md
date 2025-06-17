# Docker MCP CLI

![build](https://github.com/docker/docker-mcp/actions/workflows/ci.yml/badge.svg)

A Docker CLI plugin that provides a gateway for the Model Context Protocol (MCP), enabling seamless integration between AI language models and Docker-based MCP servers.

## What is MCP?

The [Model Context Protocol (MCP)](https://spec.modelcontextprotocol.io/) is an open protocol that standardizes how AI applications connect to external data sources and tools. It provides a secure, controlled way for language models to access and interact with various services, databases, and APIs.

## Overview

The Docker MCP CLI serves as a gateway that:

- **Manages MCP servers** running in Docker containers
- **Provides a unified interface** for AI models to access MCP servers
- **Handles authentication and security** through Docker Desktop's secrets management
- **Supports dynamic tool discovery** and configuration
- **Enables OAuth flows** for secure service connections

## Features

- 🔧 **Server Management**: List, inspect, and call MCP tools from multiple servers
- 🐳 **Container-based Servers**: Run MCP servers as Docker containers with proper isolation
- 🔐 **Secrets Management**: Secure handling of API keys and credentials via Docker Desktop
- 🌐 **OAuth Integration**: Built-in OAuth flows for service authentication
- 📋 **Server Catalog**: Manage and configure multiple MCP catalogs
- 🔍 **Dynamic Discovery**: Automatic tool, prompt, and resource discovery from running servers
- 📊 **Monitoring**: Built-in logging and call tracing capabilities

## Installation

### Prerequisites

- Docker Desktop (with MCP Toolkit feature enabled)
- Go 1.24+ (for development)

### Install as Docker CLI Plugin

```bash
# Clone the repository
git clone https://github.com/docker/docker-mcp.git
cd docker-mcp

# Build and install the plugin
make docker-mcp
```

After installation, the plugin will be available as:

```bash
docker mcp --help
```

## Usage

### Basic Commands

```bash
# Show available commands
docker mcp --help

# List all available MCP tools
docker mcp tools list

# Inspect a specific tool
docker mcp tools inspect <tool-name>

# Call a tool with arguments
docker mcp tools call <tool-name> [arguments...]

# Count available tools
docker mcp tools count
```

### Server Management

```bash
# List enabled servers
docker mcp server list

# Enable one or more servers
docker mcp server enable <server-name> [server-name...]

# Disable servers
docker mcp server disable <server-name> [server-name...]

# Get detailed information about a server
docker mcp server inspect <server-name>

# Reset (disable all servers)
docker mcp server reset
```

### Gateway Operations

```bash
# Run the MCP gateway (stdio mode)
docker mcp gateway run

# Run with specific servers only
docker mcp gateway run --servers server1,server2

# Run on TCP port instead of stdio
docker mcp gateway run --port 8080 --transport streaming

# Run with verbose logging
docker mcp gateway run --verbose --log-calls

# Run in watch mode (auto-reload on config changes)
docker mcp gateway run --watch
```

### Configuration Management

```bash
# Read current configuration
docker mcp config read

# Write new configuration
docker mcp config write '<yaml-config>'

# Reset configuration to defaults
docker mcp config reset
```

### Secrets and OAuth

```bash
# Manage secrets
docker mcp secret --help

# Handle OAuth flows
docker mcp oauth --help

# Manage access policies
docker mcp policy --help
```

### Catalog Management

```bash
# Manage server catalogs
docker mcp catalog --help
```

## Configuration

The MCP CLI uses several configuration files:

- **`docker-mcp.yaml`**: Server catalog defining available MCP servers
- **`registry.yaml`**: Registry of enabled servers
- **`config.yaml`**: Gateway configuration and options
- **`.env`**: Environment variables and secrets (optional)

Configuration files are typically stored in `~/.docker/mcp/`.

## Development

### Building from Source

```bash
# Install dependencies
go mod download

# Build the binary
make docker-mcp

# Cross-compile for all platforms
make docker-mcp-cross

# Run tests
make unit-tests
```

### Code Quality

```bash
# Format code
make format

# Run linter
make lint
```

### Docker Development

```bash
# Build development image
docker buildx build --target=format -o . .

# Run linter in container
docker buildx build --target=lint --platform=linux,darwin,windows .
```

## Architecture

The Docker MCP CLI implements a gateway pattern:

```
AI Client → MCP Gateway → MCP Servers (Docker Containers)
```

- **AI Client**: Language model or AI application
- **MCP Gateway**: This CLI tool managing protocol translation and routing
- **Tool Servers**: Individual MCP servers running in Docker containers

See [docs/message-flow.md](docs/message-flow.md) for detailed message flow diagrams.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests and linting (`make unit-tests lint`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Code of Conduct

This project follows a Code of Conduct. Please review it in [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Support

- 📖 [MCP Specification](https://spec.modelcontextprotocol.io/)
- 🐳 [Docker Desktop Documentation](https://docs.docker.com/desktop/)
- 🐛 [Report Issues](https://github.com/docker/docker-mcp/issues)
- 💬 [Discussions](https://github.com/docker/docker-mcp/discussions)
