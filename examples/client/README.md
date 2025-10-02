# Using the MCP Gateway with Docker Compose

This example shows how to call the MCP Gateway from a python client:

+ Doesn't rely on the MCP Toolkit UI. Can run anywhere, even if Docker Desktop is not available.
+ Defines the list of enabled servers from the gateway's command line, with `--server`
+ Uses the online Docker MCP Catalog hosted at http://desktop.docker.com/mcp/catalog (v2 by default, v3 when `mcp-oauth-dcr` feature is enabled).
+ Uses the latest http streaming transport.

## How to run

```console
docker compose up --build
```
