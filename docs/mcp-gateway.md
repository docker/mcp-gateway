# Docker MCP Gateway

Running MCP Servers in Docker Containers is robust and secure. 

See [Why running MCP Servers in Container is more secure](security.md)

## How to run the MCP Gateway?

Start up an MCP Gateway. This can be used for one client, or to service multiple clients if using either `sse` or `streaming` transports.

```bash
# Run the MCP gateway (stdio)
docker mcp gateway run

# Run the MCP gateway (streaming)
docker mcp gateway run --port 8080 --transport streaming

# Run with specific servers only, and select all tools from server1 and just tool2 from server2
docker mcp gateway run --servers server1,server2 --tools server1:* --tools server2:tool2

# Run a fallback secret lookup - lookup desktop secret first and the fallback to a local .env file
docker mcp gateway run --secrets=docker-desktop:./.env

# Run with verbose logging
docker mcp gateway run --verbose --log-calls

# Run in watch mode (auto-reload on config changes)
docker mcp gateway run --watch

# Run a standalone dockerized MCP server (no catalog required)
docker mcp gateway run --server docker.io/namespace/repository:latest

# Run with a profile (requires profiles feature to be enabled)
docker mcp gateway run --profile my-working-set
```

See [Profiles](profiles.md) for more information about organizing servers into reusable collections.

## How to connect to an MCP Client?

A typical usage looks like this Claude Desktop configuration:

```
{
    "mcpServers": {
        "MCP_DOCKER": {
            "command": "docker",
            "args": ["mcp", "gateway", "run"]
        }
    }
}
```

## How to run the MCP Gateway with Docker Compose?

The simplest way to tun the MCP Gateway with Docker Compose is with this kind of compose file:

```
services:
  gateway:
    image: docker/mcp-gateway
    command:
      - --servers=duckduckgo
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
```

### What does it do?

+ Starts an MCP Gateway for other services to use. Think AI Agents.
+ Work independently from Docker Desktop's MCP Toolkit. It can run anywhere there's a Docker engine.
+ Defines the list of enabled servers from the gateway's command line, with `--servers`
+ Uses the online Docker MCP Catalog (v2: https://desktop.docker.com/mcp/catalog/v2/catalog.yaml by default, v3: https://desktop.docker.com/mcp/catalog/v3/catalog.yaml when `mcp-oauth-dcr` feature is enabled).

### How to run

```console
docker compose up
```

## More examples

See [Examples](examples/README.md)

## Complete set of command line flags

See the generated reference: [`docker mcp gateway run`](generator/reference/mcp_gateway_run.md).

It is regenerated from the command definitions by `make docs` and checked in CI, so unlike
a pasted `--help` dump it cannot drift.

## Troubleshooting

Look at our [Troubleshooting Guide](/docs/troubleshooting.md)\n\n## Per-server resource limits

Set CPU and memory limits for individual containerized MCP servers with catalog
`resources` entries or explicit `--server-cpus` / `--server-memory` overrides.
See [Per-server container resource limits](server-resources.md) for precedence,
validation, and examples.
