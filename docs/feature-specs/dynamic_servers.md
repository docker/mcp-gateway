# Dynamic Server Management

## Overview

The Dynamic Server Management feature allows users to dynamically add, remove, and configure MCP servers at runtime without restarting the gateway. This feature is enabled by the `dynamic-tools` feature flag and provides a set of internal management tools that can be called by AI clients.

## Feature Flag

This feature is controlled by the `dynamic-tools` feature flag:

```bash
# Enable dynamic tools feature
docker mcp feature enable dynamic-tools

# Check if enabled
docker config view | grep dynamic-tools
```

When enabled, the gateway adds five internal management tools to the available tool set.

## Available Tools

### 1. mcp-find

**Purpose**: Search for MCP servers in the current catalog by name or description.

**Parameters**:
- `query` (required): Search query to find servers by name or description (case-insensitive)
- `limit` (optional): Maximum number of results to return (default: 10)

**Example Usage**:
```json
{
  "name": "mcp-find",
  "arguments": {
    "query": "filesystem",
    "limit": 5
  }
}
```

**Response**: Returns matching servers with their details including name, description, required secrets, config schema, and long-lived status.

### 2. mcp-add

**Purpose**: Add a new MCP server to the registry and reload the configuration.

**Parameters**:
- `name` (required): Name of the MCP server to add to the registry (must exist in catalog)

**Example Usage**:
```json
{
  "name": "mcp-add",
  "arguments": {
    "name": "filesystem"
  }
}
```

**Behavior**:
- Checks if the server exists in the catalog
- Adds the server to the active server list (avoiding duplicates)
- Fetches updated secrets for the new server
- Reloads the gateway configuration
- Returns success/error message

### 3. mcp-remove

**Purpose**: Remove an MCP server from the registry and reload the configuration.

**Parameters**:
- `name` (required): Name of the MCP server to remove from the registry

**Example Usage**:
```json
{
  "name": "mcp-remove",
  "arguments": {
    "name": "filesystem"
  }
}
```

**Behavior**:
- Removes the server from the active server list
- Reloads the gateway configuration
- Returns success message

### 4. mcp-official-registry-import

**Purpose**: Import MCP servers from an official registry URL.

**Parameters**:
- `url` (required): URL to fetch the official registry JSON from (must be HTTP/HTTPS)

**Example Usage**:
```json
{
  "name": "mcp-official-registry-import",
  "arguments": {
    "url": "https://registry.example.com/servers.json"
  }
}
```

**Behavior**:
- Validates the URL format
- Fetches server definitions via HTTP GET
- Adds imported servers to the local catalog
- Automatically enables imported servers
- Reloads the gateway configuration
- Returns detailed summary of each imported server including:
  - Server name and description
  - Docker image information  
  - Required secrets with configuration warnings
  - Available configuration schemas
  - Long-lived server indicators
  - Ready-to-use server list

### 5. mcp-config-set

**Purpose**: Set configuration values for MCP servers.

**Parameters**:
- `server` (required): Name of the MCP server to configure
- `key` (required): Configuration key to set
- `value` (required): Configuration value to set (can be string, number, boolean, or object)

**Example Usage**:
```json
{
  "name": "mcp-config-set",
  "arguments": {
    "server": "filesystem",
    "key": "allowed_paths",
    "value": ["/home/user/documents", "/tmp"]
  }
}
```

**Behavior**:
- Creates or updates server configuration
- Reloads the gateway configuration to apply changes
- Returns success message with old/new values

## Implementation Details

### Secret Management

When servers are added dynamically, the system automatically:

1. **Fetches required secrets**: Calls `readDockerDesktopSecrets()` with the updated server list
2. **Updates configuration**: Replaces the existing secrets map with newly fetched secrets
3. **Handles errors gracefully**: Logs warnings if secret fetching fails but continues operation

### Duplicate Prevention

The `mcp-add` tool ensures server name uniqueness by:
- Checking existing server names before adding
- Only appending new servers if not already present
- Maintaining the original order of servers

### Configuration Reloading

All management tools trigger configuration reloading via `reloadConfiguration()`:
- Updates tool, prompt, and resource registrations
- Refreshes capabilities with the MCP server
- Applies new secrets and configurations
- Maintains session state for existing connections

## Use Cases

### 1. Development Workflow
```
AI: I need to work with files. Let me find filesystem tools.
Tool: mcp-find -> query: "filesystem"
AI: Great! I'll add the filesystem server.
Tool: mcp-add -> name: "filesystem"
AI: Now I can use file operations.
```

### 2. Importing Official Servers
```
AI: I want to use servers from the official registry.
Tool: mcp-official-registry-import -> url: "https://registry.mcp.dev/servers.json"
AI: Successfully imported 12 servers. I can now use them.
```

### 3. Server Configuration
```
AI: I need to configure the filesystem server with specific paths.
Tool: mcp-config-set -> server: "filesystem", key: "allowed_paths", value: ["/workspace"]
AI: Configuration updated. The server now has restricted access.
```

## Security Considerations

- **Catalog validation**: Only servers that exist in the catalog can be added
- **Secret management**: Secrets are fetched securely through Docker Desktop's secrets API
- **Configuration isolation**: Server configurations are isolated and validated
- **URL validation**: Official registry imports require valid HTTP/HTTPS URLs

## Logging

The feature provides detailed logging for:
- Server additions and removals
- Configuration changes
- Secret fetching operations
- Import operations from external registries
- Warnings for duplicate servers or failed operations

## Error Handling

- **Missing servers**: Returns helpful error messages when servers aren't found in catalog
- **Secret failures**: Logs warnings but continues operation
- **Configuration errors**: Returns detailed error messages for troubleshooting
- **Network failures**: Handles HTTP errors gracefully for registry imports

## Future Enhancements

Potential future improvements include:
- Bulk server operations
- Server dependency management
- Configuration validation
- Server status monitoring
- Rollback capabilities