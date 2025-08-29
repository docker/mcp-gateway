# Using the MCP Gateway with a Custom Catalog

This example shows how you can use your own custom catalog in the MCP Gateway:

+ Defines a custom catalog `catalog.yaml` from which servers can be chosen from.
+ Picks the server `duckduckgo` from the custom catalog.
+ Uses SSE for the transport and can be connected to via `http://localhost:8811/sse`.

## How to run with compose

```console
docker compose up
```

Add client services, like Agents, that connect with the `sse` protocol on port `8811`.

## Running without Compose

```console
docker mcp gateway run --catalog=./catalog.yaml --servers=duckduckgo --transport=sse --port=8811
```
