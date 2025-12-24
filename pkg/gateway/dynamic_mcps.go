package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"

	"github.com/docker/mcp-gateway/pkg/telemetry"
)

// mcpFindTool implements a tool for finding MCP servers in the catalog
func (g *Gateway) createMcpFindTool(_ Configuration, handler mcp.ToolHandler) *ToolRegistration {
	tool := &mcp.Tool{
		Name: "mcp-find",
		Description: `Find MCP servers in the current catalog by name, title, or description.
If the user is looking for new capabilities, use this tool to search the MCP catalog for servers that should potentially be enabled.
This will not enable the server but will return information about servers that could be enabled.
If we find an mcp server, it can be added with the mcp-add tool, and configured with mcp-config-set.`,
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"query": {
					Type:        "string",
					Description: "Search query to find servers by name, title, or description (case-insensitive)",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results to return (default: 10)",
				},
			},
			Required: []string{"query"},
		},
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-find", handler),
	}
}

func (g *Gateway) createMcpAddTool(clientConfig *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name: "mcp-add",
		Description: `Add a new MCP server to the session. 
The server must exist in the catalog.`,
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the MCP server to add to the registry (must exist in catalog)",
				},
				"activate": {
					Type:        "boolean",
					Description: "Activate all of the server's tools in the current session",
					Default:     json.RawMessage("true"),
				},
			},
			Required: []string{"name"},
		},
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-add", addServerHandler(g, clientConfig)),
	}
}

// mcpConfigSetTool implements a tool for setting configuration values for MCP servers
func (g *Gateway) createMcpConfigSetTool(_ *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name: "mcp-config-set",
		Description: `Set configuration for an MCP server. 
The config object will be validated against the server's config schema. If validation fails, the error message will include the correct schema.`,
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"server": {
					Type:        "string",
					Description: "Name of the MCP server to configure",
				},
				"config": {
					Type:        "object",
					Description: "Configuration object for the server. This will be validated against the server's config schema.",
				},
			},
			Required: []string{"server", "config"},
		},
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-config-set", configSetHandler(g)),
	}
}

func (g *Gateway) createMcpCreateProfileTool(_ *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name: "mcp-create-profile",
		Description: `Create or update a profile with the current gateway state.
A profile is a snapshot of all currently enabled servers and their configurations.
If a profile with the given name already exists, it will be updated with the current state.`,
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the profile to create or update",
				},
			},
			Required: []string{"name"},
		},
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-create-profile", createProfileHandler(g)),
	}
}

func (g *Gateway) createMcpActivateProfileTool(clientConfig *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name: "mcp-activate-profile",
		Description: `Activate a saved profile by loading its servers into the current gateway.
This will merge the profile's servers with currently active servers.
Servers that are already active will be skipped.`,
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Profile ID to activate",
				},
			},
			Required: []string{"name"},
		},
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-activate-profile", activateProfileHandler(g, clientConfig)),
	}
}

func (g *Gateway) createCodeModeTool(_ *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name: "code-mode",
		Description: `Create a JavaScript-enabled tool that combines multiple MCP server tools. 
This allows you to write scripts that call multiple tools and combine their results.
Use the mcp-find tool to find servers and make sure they are are ready with the mcp-add tool. When running
mcp-add, we don't have to activate the tools.
`,
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"servers": {
					Type:        "array",
					Description: "List of MCP server names whose tools should be available in the JavaScript environment",
					Items: &jsonschema.Schema{
						Type: "string",
					},
				},
				"name": {
					Type:        "string",
					Description: "Name for the new code-mode tool (will be prefixed with 'code-mode-')",
				},
			},
			Required: []string{"servers", "name"},
		},
	}
	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("code-mode", addCodemodeHandler(g)),
	}
}

// mcpRemoveTool implements a tool for removing servers from the registry
func (g *Gateway) createMcpRemoveTool() *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-remove",
		Description: "Remove an MCP server from the registry and reload the configuration. This will disable the server.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the MCP server to remove from the registry",
				},
			},
			Required: []string{"name"},
		},
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-remove", removeServerHandler(g)),
	}
}

//nolint:unused
func (g *Gateway) createMcpRegistryImportTool(configuration Configuration, _ *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-registry-import",
		Description: "Import MCP servers from an MCP registry URL. Fetches server definitions via HTTP GET and adds them to the local catalog.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"url": {
					Type:        "string",
					Description: "URL to fetch the server details JSON (must be a valid HTTP/HTTPS URL)",
				},
			},
			Required: []string{"url"},
		},
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-registry-import", registryImportHandler(g, configuration)),
	}
}

// createMcpExecTool implements a tool for executing tools that exist in the current session
// but may not be returned from listTools calls
func (g *Gateway) createMcpExecTool() *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-exec",
		Description: "Execute a tool that exists in the current session. This allows calling tools that may not be visible in listTools results.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the tool to execute",
				},
				"arguments": {
					Type:        "object",
					Description: "Arguments to use for the tool call.",
				},
			},
			Required: []string{"name"},
		},
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-exec", addMcpExecHandler(g)),
	}
}

//nolint:unused // mcpCatalogTool implements a tool for viewing information about the currently attached catalog
func (g *Gateway) _createMcpCatalogTool() *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-catalog",
		Description: "Summarize information about the currently attached catalog, including available servers and their configurations.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return _dockerHubLink(), nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-catalog", handler),
	}
}

// withToolTelemetry wraps a tool handler with telemetry instrumentation
func withToolTelemetry(toolName string, handler mcp.ToolHandler) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		serverName := "dynamic-mcps"

		// Debug logging to stderr
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-HANDLER] Tool call received: %s from server: %s\n", toolName, serverName)
		}

		// Start telemetry span for tool call
		startTime := time.Now()
		serverType := "dynamic"

		// Build span attributes
		spanAttrs := []attribute.KeyValue{
			attribute.String("mcp.server.name", serverName),
			attribute.String("mcp.server.type", serverType),
		}

		ctx, span := telemetry.StartToolCallSpan(ctx, toolName, spanAttrs...)
		defer span.End()

		// Record tool call counter
		telemetry.ToolCallCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("mcp.server.name", serverName),
				attribute.String("mcp.server.type", serverType),
				attribute.String("mcp.tool.name", toolName),
				attribute.String("mcp.client.name", req.Session.InitializeParams().ClientInfo.Name),
			),
		)

		// Execute the wrapped handler
		result, err := handler(ctx, req)

		// Record duration
		duration := time.Since(startTime).Milliseconds()
		telemetry.ToolCallDuration.Record(ctx, float64(duration),
			metric.WithAttributes(
				attribute.String("mcp.server.name", serverName),
				attribute.String("mcp.server.type", serverType),
				attribute.String("mcp.tool.name", toolName),
				attribute.String("mcp.client.name", req.Session.InitializeParams().ClientInfo.Name),
			),
		)

		if err != nil {
			// Record error in telemetry
			telemetry.RecordToolError(ctx, span, serverName, serverType, toolName)
			span.SetStatus(codes.Error, "Tool execution failed")
			return nil, err
		}

		span.SetStatus(codes.Ok, "")
		return result, nil
	}
}
