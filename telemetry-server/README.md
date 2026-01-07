# Telemetry MCP Server

This directory contains the default telemetry MCP server used by the MCP gateway. The server receives telemetry data via MCP tool calls and records metrics using OpenTelemetry.

## Using a Custom Telemetry Server

You can provide your own telemetry MCP server by using the `--telemetry-mcp-server-host` and `--telemetry-mcp-server-port` flags when running the gateway:

```bash
docker mcp gateway run --telemetry-mcp-server-host my-telemetry-server.example.com --telemetry-mcp-server-port 8080
```

Your custom server must implement the MCP protocol with streaming HTTP transport and expose the tool calls documented below.

## Required Tool Calls

Your telemetry server must implement the following MCP tools. All tools should return a successful response (the gateway does not use the response content).

### Gateway Lifecycle

#### `record-gateway-start`

Records when the gateway starts.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `transport_mode` | string | The transport mode used by the gateway (e.g., "sse", "stdio") |

---

#### `record-initialize`

Records when a client initializes a connection.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `client_name` | string | Name of the connecting client |
| `client_version` | string | Version of the connecting client |

---

### Tool Operations

#### `record-tool-call`

Records when a tool is called.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `server_name` | string | Name of the MCP server handling the tool |
| `server_type` | string | Type of server (e.g., "docker", "sse", "streaming") |
| `tool_name` | string | Name of the tool being called |
| `client_name` | string | Name of the client making the call |

---

#### `record-tool-duration`

Records the duration of a tool call.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `server_name` | string | Name of the MCP server |
| `server_type` | string | Type of server |
| `tool_name` | string | Name of the tool |
| `client_name` | string | Name of the client |
| `duration_ms` | float64 | Duration of the call in milliseconds |

---

#### `record-tool-error`

Records when a tool call fails.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `server_name` | string | Name of the MCP server |
| `server_type` | string | Type of server |
| `tool_name` | string | Name of the tool that failed |

---

#### `record-list-tools`

Records when a client lists available tools.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `client_name` | string | Name of the client |

---

#### `record-tools-discovered`

Records the number of tools discovered from a server.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `server_name` | string | Name of the MCP server |
| `tool_count` | int64 | Number of tools discovered |

---

### Prompt Operations

#### `record-prompt-get`

Records when a prompt is retrieved.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `prompt_name` | string | Name of the prompt |
| `server_name` | string | Name of the MCP server |
| `client_name` | string | Name of the client |

---

#### `record-prompt-duration`

Records the duration of a prompt operation.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `prompt_name` | string | Name of the prompt |
| `server_name` | string | Name of the MCP server |
| `client_name` | string | Name of the client |
| `duration_ms` | float64 | Duration in milliseconds |

---

#### `record-prompt-error`

Records when a prompt operation fails.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `prompt_name` | string | Name of the prompt |
| `server_name` | string | Name of the MCP server |
| `error_type` | string | Type of error that occurred |

---

#### `record-list-prompts`

Records when a client lists available prompts.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `client_name` | string | Name of the client |

---

#### `record-prompts-discovered`

Records the number of prompts discovered from a server.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `server_name` | string | Name of the MCP server |
| `prompt_count` | int64 | Number of prompts discovered |

---

### Resource Operations

#### `record-resource-read`

Records when a resource is read.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `resource_uri` | string | URI of the resource |
| `server_name` | string | Name of the MCP server |
| `client_name` | string | Name of the client |

---

#### `record-resource-duration`

Records the duration of a resource operation.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `resource_uri` | string | URI of the resource |
| `server_name` | string | Name of the MCP server |
| `client_name` | string | Name of the client |
| `duration_ms` | float64 | Duration in milliseconds |

---

#### `record-resource-error`

Records when a resource operation fails.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `resource_uri` | string | URI of the resource |
| `server_name` | string | Name of the MCP server |
| `error_type` | string | Type of error that occurred |

---

#### `record-list-resources`

Records when a client lists available resources.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `client_name` | string | Name of the client |

---

#### `record-resources-discovered`

Records the number of resources discovered from a server.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `server_name` | string | Name of the MCP server |
| `resource_count` | int64 | Number of resources discovered |

---

### Resource Template Operations

#### `record-resource-template-read`

Records when a resource template is read.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `uri_template` | string | URI template pattern |
| `server_name` | string | Name of the MCP server |
| `client_name` | string | Name of the client |

---

#### `record-resource-template-duration`

Records the duration of a resource template operation.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `uri_template` | string | URI template pattern |
| `server_name` | string | Name of the MCP server |
| `client_name` | string | Name of the client |
| `duration_ms` | float64 | Duration in milliseconds |

---

#### `record-resource-template-error`

Records when a resource template operation fails.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `uri_template` | string | URI template pattern |
| `server_name` | string | Name of the MCP server |
| `error_type` | string | Type of error that occurred |

---

#### `record-list-resource-templates`

Records when a client lists available resource templates.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `client_name` | string | Name of the client |

---

#### `record-resource-templates-discovered`

Records the number of resource templates discovered from a server.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `server_name` | string | Name of the MCP server |
| `template_count` | int64 | Number of templates discovered |

---

### Catalog Operations

#### `record-catalog-operation`

Records a catalog operation (load, refresh, etc.).

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `operation` | string | Type of operation (e.g., "load", "refresh") |
| `catalog_name` | string | Name of the catalog |
| `duration_ms` | float64 | Duration in milliseconds |
| `success` | bool | Whether the operation succeeded |

---

#### `record-catalog-servers`

Records the number of servers in a catalog.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `catalog_name` | string | Name of the catalog |
| `server_count` | int64 | Number of servers in the catalog |

---

## Debugging

Set the `DOCKER_MCP_TELEMETRY_DEBUG` environment variable to enable debug logging for the default telemetry server:

```bash
DOCKER_MCP_TELEMETRY_DEBUG=1 docker mcp gateway run
```
