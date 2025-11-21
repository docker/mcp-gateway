# Docker MCP Gateway

Running MCP Servers in Docker Containers is robust and secure. 

See [Why running MCP Servers in Container is more secure](security.md)

## How to run the MCP Gateway?

Start up an MCP Gateway. This can be used for one client, or to service multiple clients if using either `sse` or `streaming` transports.

```bash
# Run with a profile (stdio)
docker mcp gateway run --profile my-profile

# Run the MCP gateway (stdio) - uses the profile with ID "default"
docker mcp gateway run

# Run the MCP gateway (streaming)
docker mcp gateway run --profile my-profile --port 8080 --transport streaming

# Run with verbose logging
docker mcp gateway run  --profile my-profile --verbose --log-calls

# Run in watch mode (auto-reload on config changes)
docker mcp gateway run --profile my-profile --watch

# Run a standalone dockerized MCP server (no catalog required)
docker mcp gateway run --server docker.io/namespace/repository:latest
```

See [Profiles](profiles.md) for more information about organizing servers into reusable collections.

## How to connect to an MCP Client?

A typical usage looks like this Claude Desktop configuration:

```
{
    "mcpServers": {
        "MCP_DOCKER": {
            "command": "docker",
            "args": ["mcp", "gateway", "run", "--profile", "my-profile"]
        }
    }
}
```

## How to run the MCP Gateway with Docker Compose?

*Note: This is using a deprecated method of working with the gateway. Profile support in compose is coming soon.*

The simplest way to tun the MCP Gateway with Docker Compose is with this kind of compose file:

```
services:
  gateway:
    image: docker/mcp-gateway
    command:
      - --servers=duckduckgo
      - --catalog=/mcp/catalog.yaml
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./catalog.yaml:/mcp/catalog.yaml
```

### What does it do?

+ Starts an MCP Gateway for other services to use. Think AI Agents.
+ Work independently from Docker Desktop's MCP Toolkit. It can run anywhere there's a Docker engine.
+ Defines the list of enabled servers from the gateway's command line, with `--server`
+ Uses the online Docker MCP Catalog (v2: http://desktop.docker.com/mcp/catalog/v2/catalog.yaml by default, v3: http://desktop.docker.com/mcp/catalog/v3/catalog.yaml when `mcp-oauth-dcr` feature is enabled).

### How to run

```console
docker compose up
```

## More examples

See [Examples](examples/README.md)

## Complete set of command line flags

```
Docker MCP Toolkit's CLI - Manage your MCP servers and clients.

Usage: docker mcp gateway run

Flags:
      --profile string                    Profile ID to use (incompatible with --servers, --enable-all-servers, --catalog, --registry, --config, --tools-config, --secrets, --oci-ref, --mcp-registry)
      --block-network             Block tools from accessing forbidden network resources
      --block-secrets             Block secrets from being/received sent to/from tools (default true)
      --catalog string            path to the docker-mcp.yaml catalog (absolute or relative to ~/.docker/mcp/catalogs/) (default "docker-mcp.yaml")
      --config string             path to the config.yaml (absolute or relative to ~/.docker/mcp/) (default "config.yaml")
      --cpus int                  CPUs allocated to each MCP Server (default is 1) (default 1)
      --dry-run                   Start the gateway but do not listen for connections (useful for testing the configuration)
      --interceptor stringArray   List of interceptors to use (format: when:type:path, e.g. 'before:exec:/bin/path')
      --keep                      Keep stopped containers
      --log-calls                 Log calls to the tools (default true)
      --memory string             Memory allocated to each MCP Server (default is 2Gb) (default "2Gb")
      --port int                  TCP port to listen on (default is to listen on stdio)
      --registry string           path to the registry.yaml (absolute or relative to ~/.docker/mcp/) (default "registry.yaml")
      --secrets docker-desktop    colon separated paths to search for secrets. Can be docker-desktop or a path to a .env file (default to using Docker Deskop's secrets API) (default "docker-desktop")
      --servers strings           names of the servers to enable (if non empty, ignore --registry flag)
      --tools strings             List of tools to enable
      --transport string          stdio, sse or streaming (default is stdio) (default "stdio")
      --verbose                   Verbose output
      --verify-signatures         Verify signatures of the server images
      --watch                     Watch for changes and reconfigure the gateway (default true)
```

## Troubleshooting

Look at our [Troubleshooting Guide](/docs/troubleshooting.md)
