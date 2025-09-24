## Self-Contained MCP Docker Image

Before publishing an MCP server image, users can still run the MCP.

```bash
docker mcp gateway run --servers docker://namespace/repository:latest
```

* the `docker://` prefix is required.

## no catalog required

In the example above, the namespace/repository:latest is not yet published, but it is available.

The image must contain the following label:

```
LABEL io.docker.server.metadata="{... server metadata ...}"
```

### Example

Build the image with the server metadata added to the label.

```bash
docker build \
  --label "io.docker.server.metadata=$(cat <<'EOF'
name: my-mcp-server
description: "Custom MCP server for things"
command: ["python", "/app/server.py"]
env:
  - name: LOG_LEVEL
    value: "{{my-mcp-server.log-levell}}"
  - name: DEBUG
    value: "false"
secrets:
  - name: my-mcp-server.API_KEY
    env: API_KEY
config:
  - name: my-mcp-server
    type: object
    properties:
      log-level:
        type: string
    required:
      - level
EOF
)" \
  -t namespace/repository:latest .
```

* the `image` property is not required. This image is self-describing.
