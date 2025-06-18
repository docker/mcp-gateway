# Using the MCP Gateway with Docker Compose

This is a simple example of running the MCP Gateway with Docker Compose that proxies to other remote MCP servers:

+ Doesn't rely on the MCP Toolkit UI or the Docker socket. Can run anywhere, even if Docker Desktop is not available.
+ Defines the list of enabled servers from the gateway's command line, with `--server`.
+ Uses a custom catalog that lists a single remote MCP server.
+ Uses SSE for the transport and can be connected to via `http://localhost:8811/sse`.

## How to run

```console
docker compose up
```

Add client services, like Agents, that connect with the `sse` protocol on port `8811`.
If needed, the protocol can be changed to `stdio` over tcp or `streaming`.
