# Using the MCP Gateway with Docker Compose

This is a very minimalist example of running the MCP Gateway with Docker Compose:

+ Doesn't rely on the MCP Toolkit UI. Can run anywhere, even if Docker Desktop is not available.
+ Defines the list of enabled servers from the gateway's command line, with `--server`
+ Doesn't define any secret.
+ Uses the online Docker MCP Catalog hosted on http://desktop.docker.com/mcp/catalog/v2/catalog.yaml.

## How to run

```console
docker compose up
```

Add client services, like Agents, that connect with the `stdio` protocol on port `8811`.
If needed, the protocol can be changed to `sse` or `streaming` with `--transport=sse` or `--transport=streaming`.
