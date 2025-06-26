# Using the MCP Gateway with Docker Compose

This is a simple example of running the `filesystem` MCP Gateway with Docker Compose:

+ Doesn't rely on the MCP Toolkit UI. Can run anywhere, even if Docker Desktop is not available.
+ Defines the list of enabled servers from the gateway's command line, with `--server`
+ Doesn't define any secret.
+ Uses the online Docker MCP Catalog hosted on http://desktop.docker.com/mcp/catalog/v2/catalog.yaml.

## How to run

```console
docker compose up
```
