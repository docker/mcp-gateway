# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is the **Docker MCP Gateway** - a CLI plugin that enables easy and secure running of Model Context Protocol (MCP) servers through Docker containers. The plugin acts as a gateway between AI clients and containerized MCP servers, providing isolation, security, and management capabilities.

## Architecture

The codebase follows a gateway pattern where:
- **AI Client** connects to the **MCP Gateway** 
- **MCP Gateway** (this CLI) manages multiple **MCP Servers** running in Docker containers

Key architectural components:
- **Gateway**: Core routing and protocol translation (`cmd/docker-mcp/internal/gateway/`)
- **Client Management**: Handles connections to AI clients (`cmd/docker-mcp/client/`)
- **Server Management**: Manages MCP server lifecycle (`cmd/docker-mcp/server/`)
- **Catalog System**: Manages available MCP servers (`cmd/docker-mcp/catalog/`)
- **Security**: Secrets management and OAuth flows (`cmd/docker-mcp/secret-management/`, `cmd/docker-mcp/oauth/`)

## Development Commands

### Building
```bash
# Build the CLI plugin locally
make docker-mcp

# Cross-compile for all platforms
make docker-mcp-cross
```

### Testing
```bash
# Run all tests
make test

# Run integration tests specifically
make integration

# Run long-lived integration tests
go test -count=1 ./... -run 'TestLongLived'

# Run specific tests by pattern
go test -count=1 ./... -run 'TestIntegration'

# Run a single test file
go test ./cmd/docker-mcp/server/server_test.go

# Run tests with coverage
go test -cover ./...
```

### Code Quality
```bash
# Format code
make format

# Run linter
make lint

# Run linter for specific platform
make lint-linux
make lint-darwin

# Run Go vet (static analysis)
go vet ./...

# Run Go mod tidy to clean dependencies
go mod tidy
```

## Project Structure

- `cmd/docker-mcp/` - Main CLI application entry point
- `cmd/docker-mcp/internal/gateway/` - Core gateway implementation with client pooling, proxy management, and transport handling
- `cmd/docker-mcp/internal/docker/` - Docker integration for container management
- `cmd/docker-mcp/internal/mcp/` - MCP protocol implementations (stdio, SSE)
- `cmd/docker-mcp/internal/desktop/` - Docker Desktop integration and authentication
- `cmd/docker-mcp/catalog/` - Server catalog management commands
- `cmd/docker-mcp/client/` - Client configuration and connection management
- `cmd/docker-mcp/server/` - Server lifecycle management commands  
- `cmd/docker-mcp/tools/` - Tool execution and management
- `examples/` - Usage examples and compose configurations
- `docs/` - Technical documentation

## Key Configuration Files

The CLI uses these configuration files (typically in `~/.docker/mcp/`):
- `docker-mcp.yaml` - Server catalog definitions
- `registry.yaml` - Registry of enabled servers  
- `config.yaml` - Gateway configuration and options

## Important Patterns

### Transport Modes
The gateway supports multiple transport modes:
- `stdio` - Standard input/output (default)
- `streaming` - HTTP streaming for multiple clients
- `sse` - Server-sent events

### Security Features
- Container isolation for MCP servers
- Secrets management via Docker Desktop
- OAuth flow handling
- API key and credential protection
- Call interception and logging

### Client Integration
The system integrates with various AI clients:
- VS Code / Cursor
- Claude Desktop  
- Continue Dev
- Custom MCP clients

Configuration files for different clients are automatically managed in `cmd/docker-mcp/client/testdata/`.

## CLI Plugin Development

This is a Docker CLI plugin written in Go 1.24+. Key development patterns:

### Plugin Installation
The plugin is installed as `docker-mcp` and becomes available as `docker mcp <command>`. The Makefile handles building and installing to the correct Docker CLI plugins directory (`~/.docker/cli-plugins/`).

### Command Structure
Commands follow the Cobra CLI pattern with the main command tree defined in `cmd/docker-mcp/commands/`. Each major command area (catalog, server, client, etc.) has its own file.

### Configuration Management
The CLI uses YAML configuration files stored in `~/.docker/mcp/`:
- Server definitions are loaded from catalog files
- Runtime configuration is managed through config.yaml
- Server enablement tracked in registry.yaml

### Container Lifecycle
MCP servers run as Docker containers with proper lifecycle management:
- Images are pulled and validated before use  
- Containers have consistent naming patterns
- Health checks and logging are built-in
- Proper cleanup on shutdown

### Testing Patterns
- Integration tests require Docker daemon
- Long-lived tests run actual container scenarios
- Mock configurations in testdata directories
- Use `go test -count=1` to disable test caching