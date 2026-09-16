package gateway

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/telemetry"
	"github.com/docker/mcp-gateway/pkg/toolschema"
)

// toolRegistration builds the gateway's own registration for a tool discovered
// on an upstream server.
//
// The gateway must not write through the *mcp.Tool it was handed: that value,
// and the schema maps reachable from it, are owned by the upstream client
// session's cached tool list and are read concurrently across servers. Every
// rewrite here lands on the local copy.
func (g *Gateway) toolRegistration(ctx context.Context, serverConfig *catalog.ServerConfig, tool *mcp.Tool, prefix string) ToolRegistration {
	prefixedTool := *tool
	prefixedTool.Name = prefixToolName(prefix, tool.Name)
	g.normalizeToolSchemaDialects(ctx, &prefixedTool, serverConfig.Name)

	return ToolRegistration{
		ServerName: serverConfig.Name,
		Tool:       &prefixedTool,
		Handler: withMCPServerToolTelemetry(
			serverConfig,
			g.withInvokePolicy(
				serverConfig.Name,
				tool.Name,
				g.mcpServerToolHandler(serverConfig.Name, g.mcpServer, tool.Annotations, tool.Name),
			),
		),
	}
}

// normalizeToolSchemaDialects translates a tool's schemas into the JSON Schema
// 2020-12 dialect.
//
// MCP only requires a client to support 2020-12, and several widely used
// clients validate with a 2020-12-only validator, so a tool declaring draft-07
// is rejected before it is ever called. Servers built on the MCP TypeScript SDK
// declare draft-07 for every tool, because its zod converter defaults to that
// target, which makes those servers unusable through the gateway for reasons
// that have nothing to do with the gateway.
//
// tool must already be the gateway's own copy; see toolRegistration.
func (g *Gateway) normalizeToolSchemaDialects(ctx context.Context, tool *mcp.Tool, serverName string) {
	if g.PreserveToolSchemaDialect {
		return
	}

	tool.InputSchema = normalizeSchemaDialect(ctx, tool.InputSchema, serverName, tool.Name, "inputSchema")
	tool.OutputSchema = normalizeSchemaDialect(ctx, tool.OutputSchema, serverName, tool.Name, "outputSchema")
}

func normalizeSchemaDialect(ctx context.Context, schema any, serverName, toolName, field string) any {
	normalized, result := toolschema.Normalize(schema)
	switch {
	case result.Skipped:
		log.Logf("  > Relaying %s of tool %q from %s with its declared dialect: %s",
			field, toolName, serverName, result.Reason)
		telemetry.RecordToolSchemaDialect(ctx, serverName, field, "relayed")
	case result.Changed:
		telemetry.RecordToolSchemaDialect(ctx, serverName, field, "translated")
	}
	return normalized
}
