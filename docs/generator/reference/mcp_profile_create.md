# docker mcp profile create

<!---MARKER_GEN_START-->
Create a new profile that groups multiple MCP servers together.
A profile allows you to organize and manage related servers as a single unit.
Profiles are decoupled from catalogs. Servers can be:
  - MCP Registry references (e.g. http://registry.modelcontextprotocol.io/v0/servers/312e45a4-2216-4b21-b9a8-0f1a51425073)
  - OCI image references with docker:// prefix (e.g., "docker://my-server:latest"). Images must be self-describing.
  - Catalog references with catalog:// prefix (e.g., "catalog://mcp/docker-mcp-catalog/github+obsidian").
  - Local file references with file:// prefix (e.g., "file://./server.yaml").

### Options

| Name        | Type          | Default | Description                                                                                                                                                                                                        |
|:------------|:--------------|:--------|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--connect` | `stringArray` |         | Clients to connect to: mcp-client (can be specified multiple times). Supported clients: [claude-code claude-desktop cline codex continue crush cursor gemini goose gordon kiro lmstudio opencode sema4 vscode zed] |
| `--id`      | `string`      |         | ID of the profile (defaults to a slugified version of the name)                                                                                                                                                    |
| `--name`    | `string`      |         | Name of the profile (required)                                                                                                                                                                                     |
| `--server`  | `stringArray` |         | Server to include specified with a URI: https:// (MCP Registry reference) or docker:// (Docker Image reference) or catalog:// (Catalog reference) or file:// (Local file path). Can be specified multiple times.   |


<!---MARKER_GEN_END-->

