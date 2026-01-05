# Tool Registrations Serializer

This tool initializes a gateway with configured MCP servers and serializes their tool registrations to disk in JSON format.

## Purpose

The tool registration serializer is useful for:
- **Introspection**: Understanding what tools are available across all enabled servers
- **Documentation**: Generating tool catalogs for external use
- **Testing**: Validating that tools are being registered correctly
- **Integration**: Providing tool metadata to other systems that need to know about available tools

## Usage

### Basic Usage

```bash
# Serialize tool registrations from all enabled servers (in registry.yaml)
go run main.go

# Specify a custom output file
go run main.go -output my-tools.json

# Serialize only specific servers
go run main.go -server filesystem -server postgres

# Use custom configuration files
go run main.go \
  -catalog /path/to/catalog.yaml \
  -registry /path/to/registry.yaml \
  -config /path/to/config.yaml \
  -output tools.json
```

### Flags

- `-catalog <path>`: Path to MCP server catalog (default: `docker-mcp.yaml`)
- `-registry <path>`: Path to registry file with enabled servers (default: `registry.yaml`)
- `-config <path>`: Path to config file with server configurations (default: `config.yaml`)
- `-tools <path>`: Path to tools config file (default: `tools.yaml`)
- `-secrets <path>`: Path to secrets (default: `docker-desktop`)
- `-output <path>`: Output file for tool registrations (default: `tool-registrations.json`)
- `-server <name>`: Server name to include (can be repeated, omit to use all enabled servers)

### Examples

#### Example 1: Export All Enabled Tools

```bash
cd ~/.docker/mcp
go run /path/to/examples/tool_registrations/main.go
```

Output: `tool-registrations.json` with all tools from enabled servers

#### Example 2: Export Tools from Specific Servers

```bash
go run main.go \
  -server filesystem \
  -server postgres \
  -server brave-search \
  -output web-tools.json
```

#### Example 3: Use with Custom Paths

```bash
go run main.go \
  -catalog ./my-catalog.yaml \
  -registry ./my-registry.yaml \
  -config ./my-config.yaml \
  -output ./output/tools.json
```

## Output Format

The tool generates a JSON file with the following structure:

```json
{
  "tool-name": {
    "server_name": "server-name",
    "tool": {
      "name": "tool-name",
      "description": "Tool description",
      "inputSchema": {
        "type": "object",
        "properties": {
          "param1": {
            "type": "string",
            "description": "Parameter description"
          }
        },
        "required": ["param1"]
      }
    }
  }
}
```

### Example Output

```json
{
  "list_directory": {
    "server_name": "filesystem",
    "tool": {
      "name": "list_directory",
      "description": "List contents of a directory",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "Directory path to list"
          }
        },
        "required": ["path"]
      }
    }
  },
  "read_file": {
    "server_name": "filesystem",
    "tool": {
      "name": "read_file",
      "description": "Read contents of a file",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "File path to read"
          }
        },
        "required": ["path"]
      }
    }
  }
}
```

## How It Works

1. **Gateway Initialization**: Creates a gateway instance with the specified configuration
2. **Configuration Loading**: Reads server catalog, registry, and configuration files
3. **Server Connection**: Connects to each enabled MCP server
4. **Tool Discovery**: Lists all tools available from each server
5. **Registration**: Collects tool registrations from all servers
6. **Serialization**: Converts tool registrations to JSON (excluding non-serializable handler functions)
7. **Output**: Writes the JSON to the specified output file

## Notes

- The tool runs in "static" mode, so it won't pull Docker images
- Handler functions are not serialized (they are runtime-only)
- The tool respects the same configuration files as the main gateway
- Use `-server` flags to limit which servers' tools are exported
- Omitting `-server` will export tools from all enabled servers in `registry.yaml`

## Integration Example

You can use this tool in scripts to generate tool documentation:

```bash
#!/bin/bash
# Export tool registrations
go run main.go -output tools.json

# Generate markdown documentation from JSON
jq -r 'to_entries[] | "## \(.value.tool.name)\n\n**Server**: \(.value.server_name)\n\n\(.value.tool.description)\n"' tools.json > TOOLS.md
```

## Troubleshooting

### Error: "reading configuration: no such file"
Make sure you're running from the correct directory or provide absolute paths to configuration files.

### Error: "listing resources: unable to connect to server"
Ensure Docker is running and the specified servers are properly configured in your catalog and registry files.

### Empty output file
Check that you have servers enabled in your `registry.yaml` file, or specify servers explicitly with `-server` flags.
