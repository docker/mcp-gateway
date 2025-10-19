docker build \
  --label "io.modelcontextprotocol.server.name=io.github.slimslenderslacks/elicit" \
  --label "io.docker.server.metadata=$(cat <<'EOF'
name: elicit-server
description: "Custom MCP server for things"
longLived: true
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
  -t jimclark106/elicit --file test/servers/elicit/Dockerfile .

