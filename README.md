# Docker MCP Plugin and Docker MCP Gateway

![build](https://github.com/docker/mcp-gateway/actions/workflows/ci.yml/badge.svg)

The [MCP Toolkit](https://docs.docker.com/ai/mcp-catalog-and-toolkit/toolkit/), in Docker Desktop, allows
developers to configure and consume MCP servers from the [Docker MCP Catalog](https://hub.docker.com/mcp).

Underneath, the Toolkit is powered by a docker CLI plugin: `docker-mcp`. This repository is the code of this CLI plugin. It can work in Docker Desktop or independently.

The main feature of this CLI is the **Docker MCP Gateway** which allows easy and secure running and deployment of MCP servers. See [Features](#Features) for a list of all the features.

## What is MCP?

The [Model Context Protocol (MCP)](https://spec.modelcontextprotocol.io/) is an open protocol that standardizes how AI applications connect to external data sources and tools. It provides a secure, controlled way for language models to access and interact with various services, databases, and APIs.

## Overview

Developers face criticial barriers when integrating Model Context Protocol (MCP) tools into production workflows:

- **Managing MCP server lifecycle** Each local MCP sever in the catalog runs in an isolated Docker container. npx and uvx servers are granted minimal host privileges.
- **Providing a unified interface** AI models access MCP servers through a single Gateway.
- **Handling authentication and security** Keep secrets out of environment variables using Docker Desktop's secrets management.
- **Supports dynamic tool discovery** and configuration. Each MCP client (eg VS Code, Cursor, Claude Desktop, etc.) connects to the same Gateway configuration, ensuring consistency across different clients.
- **Enables OAuth flows** for MCPs that require OAuth access token service connections.

## Features

- 🐳 **Container-based Servers**: Run MCP servers as Docker containers with proper isolation.
- 🔧 **Server Management**: List, inspect, and call MCP tools, resources and prompts from multiple servers.
- 🔐 **Secrets Management**: Secure handling of API keys and credentials via Docker Desktop.
- 🌐 **OAuth Integration**: Built-in OAuth flows for service authentication.
- 📋 **Server Catalog**: Manage and configure multiple MCP catalogs.
- 🔍 **Dynamic Discovery**: Automatic tool, prompt, and resource discovery from running servers.
- 📊 **Monitoring**: Built-in logging and call tracing capabilities.

## Installation

### Prerequisites

- Docker Desktop (with MCP Toolkit feature enabled)

<div align="left">
  <img src="./img/enable_toolkit.png" width="400"/>
</div>
- Go 1.24+ (for development)

### Install as Docker CLI Plugin

The MCP cli will already be installed on recent versions of Docker Desktop but you can buildand install the latest version by following these steps:

```bash
# Clone the repository
git clone https://github.com/docker/mcp-gateway.git
cd mcp-gateway
mkdir -p "$HOME/.docker/cli-plugins/"

# Build and install the plugin
make docker-mcp
```

After installation, the plugin will be available as:

```bash
docker mcp --help
```

## Usage

### Catalog Management

Manage the catalogs available to the MCP gateway. The [default catalog](https://hub.docker.com/mcp) is available with the name 'docker-mcp'.

```bash
# Manage server catalogs
docker mcp catalog --help

# Initialize the default Docker MCP Catalog
docker mcp catalog init

# List available catalogs
docker mcp catalog ls

# Show all servers in a catalog
docker mcp catalog show docker-mcp
```

* more about [the MCP Catalog](docs/catalog.md).
* more about [importing from the OSS MCP Community Registry](docs/catalog.md#importing-from-the-oss-mcp-community-registry).

### MCP Gateway Operations

Start up an MCP Gateway. This can be used for one client, or to service multiple clients if using either `sse` or `streaming` transports.

```bash
# Run the MCP gateway (stdio)
docker mcp gateway run

# Run the MCP gateway (streaming)
docker mcp gateway run --port 8080 --transport streaming
```

* more about [the MCP Gateway](docs/mcp-gateway.md)
* [running an unpublished local image](docs/self-configured.md)

### Server Management

Enable and disable the set of MCP servers that will be available for default clients. The MCP gateway can be configured to expose different sets of servers and tools but enabling and disabling servers here impacts the default gateway configuration.

```bash
# List enabled servers
docker mcp server ls

# Enable one or more servers
docker mcp server enable <server-name> [server-name...]

# Disable servers
docker mcp server disable <server-name> [server-name...]

# Get detailed information about a server
docker mcp server inspect <server-name>

# Reset (disable all servers)
docker mcp server reset
```

### Configuration Management

Configure any MCP servers that require custom runtime configuration.

```bash
# Read current configuration
docker mcp config read

# Write new configuration
docker mcp config write '<yaml-config>'

# Reset configuration to defaults
docker mcp config reset
```

### Secrets and OAuth

Configure MCP servers that require either secrets or OAuth.

```bash
# Manage secrets
docker mcp secret --help

# Handle OAuth flows
docker mcp oauth --help

# Manage access policies
docker mcp policy --help

# export any desktop secrets needed by either server1 or server2
#   (temporary requirement to export secrets for docker cloud runs - this command
#    will no longer be required once Docker Cloud can access secret stores) 
docker mcp secret export server1 server2
```

### Tool Management

```bash
# Show available commands
docker mcp --help

# Count available tools
docker mcp tools count

# List all available MCP tools
docker mcp tools ls

# List all available MCP tools in JSON format
docker mcp tools ls --format=json

# Inspect a specific tool
docker mcp tools inspect <tool-name>

# Call a tool with arguments
docker mcp tools call <tool-name> [arguments...]
```

## Configuration

The MCP CLI uses several configuration files:

- **`docker-mcp.yaml`**: Server catalog defining available MCP servers
- **`registry.yaml`**: Registry of enabled servers
- **`config.yaml`**: Configuration per server
- **`tools.yaml`**: Enabled tools per server

Configuration files are typically stored in `~/.docker/mcp/`. This is in this directory that Docker Desktop's
MCP Toolkit with store its configuration.

## Architecture

The Docker MCP CLI implements a gateway pattern:

```
AI Client → MCP Gateway → MCP Servers (Docker Containers)
```

- **AI Client**: Language model or AI application
- **MCP Gateway**: This CLI tool managing protocol translation and routing
- **MCP Servers**: Individual MCP servers running in Docker containers

See [docs/message-flow.md](docs/message-flow.md) for detailed message flow diagrams.

## Contributing

The build instructions are available in the [contribution guide](CONTRIBUTING.md).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- 💬 [Troubleshooting](/docs/troubleshooting.md)
- 📖 [MCP Specification](https://spec.modelcontextprotocol.io/)
- 🐳 [Docker Desktop Documentation](https://docs.docker.com/desktop/)
- 🐛 [Report Issues](https://github.com/docker/mcp-gateway/issues)
- 💬 [Discussions](https://github.com/docker/mcp-gateway/discussions)
