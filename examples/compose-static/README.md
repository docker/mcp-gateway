# Using the MCP Gateway with Docker Compose in static mode

**This is Experimental**

This example shows how to run the MCP Gateway in `static` mode.
Instead of starting MCP servers on demand, through the docker API,
it starts all the enabled MCP servers in advance and keeps those Servers
up and running as long as the Compose stack runs.

## How to run

```console
docker compose up
```

