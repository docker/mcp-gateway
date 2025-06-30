# Configuration and secrets with Docker Compose

This example shows how to query a Neo4j through a Neo4j MCP Server,
through the MCP Gateway, from a python client:

+ Configure MCP Gateway with Compose Secrets and Compose Configs.
+ Use Health Check to wait for Neo4j before starting the MCP Gateway.

## How to run

```console
docker compose up --build
```
