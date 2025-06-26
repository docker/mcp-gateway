# Using the MCP Gateway with Docker Compose and the MCP Toolkit

+ `minimal` - Simplest Compose file. Just one MCP Server, without configuration or secrets.
+ `client` - A Python client connecting to the MCP Gateway over http streaming transport.
+ `secrets` - Just one MCP Server, with a secret handled in an `.env` file.
+ `remote_mcp` - Uses the gateway as a proxy to a remote MCP server.
+ `mcp_toolkit` - Connect to the MCP Toolkit and let it handle all the configuration and secrets.
+ `postgresql` - Query a PostgreSQL DB through a PostgreSQL MCP Server, through the Gateway, from a python client.
+ `docker-in-docker` - Run the MCP Gateway and the MCP server into the same Docker in Docker container.
+ `filesystem` - Run the filesystem MCP Server in Compose.