# Using the MCP Gateway with Docker Compose

This example shows how to query a PostgreSQL database through a PostgreSQL MCP Server,
through the MCP Gateway, from a python client:

+ Configure MCP Gateway with Compose Secrets.
+ Use Health Check to wait for PostgreSQL before starting the MCP Gateway.

## How to run

```console
docker compose up --build
```
