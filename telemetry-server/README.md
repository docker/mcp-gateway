# Telemetry MCP Server

This directory contains the default telemetry MCP server used by the MCP gateway. The server receives telemetry data via MCP tool calls and records metrics using OpenTelemetry.

## Architecture

The telemetry system uses a plugin architecture that decouples the gateway from the specific telemetry implementation:

```
Gateway -> telemetry.go -> Plugin Registry -> TelemetryPlugin Interface
                                                      |
                                              MCP Telemetry Adapter
                                                      |
                                              MCP Tool Calls (record-counter, record-histogram, record-gauge)
                                                      |
                                              Telemetry MCP Server
                                                      |
                                              OpenTelemetry Backend
```

This design allows:
- Easy swapping of telemetry backends without changing gateway code
- Adding new metrics without changing the plugin interface
- Testing with mock plugins

## Using a Custom Telemetry Server

You can provide your own telemetry MCP server by using the `--telemetry-mcp-server-host` and `--telemetry-mcp-server-port` flags when running the gateway:

```bash
docker mcp gateway run --telemetry-mcp-server-host my-telemetry-server.example.com --telemetry-mcp-server-port 8080
```

Your custom server must implement the MCP protocol with streaming HTTP transport and expose the three tool calls documented below.

## Required Tool Calls

Your telemetry server only needs to implement **3 generic tools**. This design mirrors the OpenTelemetry interface, making it easy to implement and allowing new metrics to be added without changing the plugin.

### `record-counter`

Records a counter metric increment. Counters are cumulative metrics that only increase (e.g., number of requests).

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `name` | string | The metric name (e.g., "mcp.tool.calls") |
| `value` | int64 | The increment value (usually 1) |
| `attributes` | map[string]string | Key-value pairs for metric dimensions |

**Example:**
```json
{
  "name": "mcp.tool.calls",
  "value": 1,
  "attributes": {
    "mcp.server.name": "my-server",
    "mcp.tool.name": "docker_ps",
    "mcp.client.name": "claude"
  }
}
```

---

### `record-histogram`

Records a value in a histogram metric. Histograms track the distribution of values (e.g., request durations).

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `name` | string | The metric name (e.g., "mcp.tool.duration") |
| `value` | float64 | The value to record (typically milliseconds for durations) |
| `attributes` | map[string]string | Key-value pairs for metric dimensions |

**Example:**
```json
{
  "name": "mcp.tool.duration",
  "value": 150.5,
  "attributes": {
    "mcp.server.name": "my-server",
    "mcp.tool.name": "docker_ps",
    "mcp.client.name": "claude"
  }
}
```

---

### `record-gauge`

Records a point-in-time value. Gauges represent values that can go up or down (e.g., current connections, discovered resources).

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `name` | string | The metric name (e.g., "mcp.tools.discovered") |
| `value` | int64 | The current value |
| `attributes` | map[string]string | Key-value pairs for metric dimensions |

**Example:**
```json
{
  "name": "mcp.tools.discovered",
  "value": 15,
  "attributes": {
    "mcp.server.origin": "my-server"
  }
}
```

---

## Standard Metric Names

The gateway sends the following metric names. Your telemetry server can use these to create appropriately-typed OpenTelemetry instruments:

### Counters (via `record-counter`)
| Metric Name | Description |
|-------------|-------------|
| `mcp.tool.calls` | Number of tool calls executed |
| `mcp.tool.errors` | Number of tool call errors |
| `mcp.gateway.starts` | Number of gateway starts |
| `mcp.initialize` | Number of client initialize calls |
| `mcp.list.tools` | Number of list tools calls |
| `mcp.catalog.operations` | Number of catalog operations |
| `mcp.prompt.gets` | Number of prompt get operations |
| `mcp.prompt.errors` | Number of prompt errors |
| `mcp.list.prompts` | Number of list prompts calls |
| `mcp.resource.reads` | Number of resource read operations |
| `mcp.resource.errors` | Number of resource errors |
| `mcp.list.resources` | Number of list resources calls |
| `mcp.resource_template.reads` | Number of resource template reads |
| `mcp.resource_template.errors` | Number of resource template errors |
| `mcp.list.resource_templates` | Number of list resource template calls |

### Histograms (via `record-histogram`)
| Metric Name | Description | Unit |
|-------------|-------------|------|
| `mcp.tool.duration` | Duration of tool call execution | ms |
| `mcp.catalog.operation.duration` | Duration of catalog operations | ms |
| `mcp.prompt.duration` | Duration of prompt operations | ms |
| `mcp.resource.duration` | Duration of resource operations | ms |
| `mcp.resource_template.duration` | Duration of resource template operations | ms |

### Gauges (via `record-gauge`)
| Metric Name | Description |
|-------------|-------------|
| `mcp.tools.discovered` | Number of tools discovered from servers |
| `mcp.catalog.servers` | Number of servers in catalogs |
| `mcp.prompts.discovered` | Number of prompts discovered |
| `mcp.resources.discovered` | Number of resources discovered |
| `mcp.resource_templates.discovered` | Number of resource templates discovered |

## Common Attributes

These attributes are commonly included with metrics:

| Attribute | Description |
|-----------|-------------|
| `mcp.server.name` | Name of the MCP server |
| `mcp.server.type` | Type of server (docker, sse, streaming, etc.) |
| `mcp.server.origin` | Origin server name |
| `mcp.tool.name` | Name of the tool |
| `mcp.client.name` | Name of the client |
| `mcp.client.version` | Version of the client |
| `mcp.prompt.name` | Name of the prompt |
| `mcp.resource.uri` | URI of the resource |
| `mcp.resource_template.uri` | URI template pattern |
| `mcp.catalog.name` | Name of the catalog |
| `mcp.catalog.operation` | Type of catalog operation |
| `mcp.catalog.success` | Whether the operation succeeded ("true"/"false") |
| `mcp.error.type` | Type of error that occurred |
| `mcp.gateway.transport` | Gateway transport mode |

## Implementing a Custom Server

Here's a minimal example of implementing a custom telemetry server in Go:

```go
package main

import (
    "context"
    "github.com/modelcontextprotocol/go-sdk/mcp"
    "go.opentelemetry.io/otel/metric"
)

func main() {
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "my-telemetry-server",
        Version: "1.0.0",
    }, nil)

    // Record counter tool
    type CounterArgs struct {
        Name       string            `json:"name"`
        Value      int64             `json:"value"`
        Attributes map[string]string `json:"attributes"`
    }
    mcp.AddTool(server, &mcp.Tool{
        Name:        "record-counter",
        Description: "Record a counter metric",
    }, func(ctx context.Context, _ *mcp.CallToolRequest, args CounterArgs) (*mcp.CallToolResult, any, error) {
        // Record to your metrics backend
        recordCounter(args.Name, args.Value, args.Attributes)
        return &mcp.CallToolResult{
            Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}},
        }, nil, nil
    })

    // Similar for record-histogram and record-gauge...
}
```

## Debugging

Set the `DOCKER_MCP_TELEMETRY_DEBUG` environment variable to enable debug logging:

```bash
DOCKER_MCP_TELEMETRY_DEBUG=1 docker mcp gateway run
```

This will show:
- `[MCP-TELEMETRY]` - Telemetry package logs
- `[MCP-TELEMETRY-ADAPTER]` - MCP adapter logs
- `[MCP-TELEMETRY-SERVER]` - Server-side logs (default server only)
