# Run the MCP Gateway in a container

The MCP Gateway can run in a container and still mount the MCP Toolkit's configuration.
The following command runs an always on Gateway on your machine. It'll even start
automatically on Docker's reboot.

## How to run

```console
docker run -d \
    -p 8811:8811 \
    --restart=always \
    --name=mcp-gateway \
    --use-api-socket \
    -v $HOME/.docker/mcp:/mcp:ro \
    docker/mcp-gateway \
    --catalog=/mcp/catalogs/docker-mcp.yaml \
    --config=/mcp/config.yaml \
    --registry=/mcp/registry.yaml \
    --tools-config=/mcp/tools.yaml \
    --secrets=docker-desktop \
    --watch=true \
    --transport=sse \
    --port=8811
```

Connect to it on `http://localhost:8811/sse`
