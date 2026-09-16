package gateway

import (
	"context"
	"sort"

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
func (g *Gateway) toolRegistration(
	ctx context.Context,
	serverConfig *catalog.ServerConfig,
	tool *mcp.Tool,
	prefix string,
	relayed *relayedDialects,
) ToolRegistration {
	prefixedTool := *tool
	prefixedTool.Name = prefixToolName(prefix, tool.Name)
	g.normalizeToolSchemaDialects(ctx, &prefixedTool, serverConfig.Name, relayed)

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
func (g *Gateway) normalizeToolSchemaDialects(
	ctx context.Context,
	tool *mcp.Tool,
	serverName string,
	relayed *relayedDialects,
) {
	if g.PreserveToolSchemaDialect {
		return
	}

	tool.InputSchema = g.normalizeSchemaDialect(ctx, tool.InputSchema, serverName, "inputSchema", relayed)
	tool.OutputSchema = g.normalizeSchemaDialect(ctx, tool.OutputSchema, serverName, "outputSchema", relayed)
}

func (g *Gateway) normalizeSchemaDialect(
	ctx context.Context,
	schema any,
	serverName, field string,
	relayed *relayedDialects,
) any {
	normalized, result := toolschema.Normalize(schema)
	switch {
	case result.Skipped:
		relayed.record(field, result.Reason)
		telemetry.RecordToolSchemaDialect(ctx, serverName, field, "relayed")
	case result.UnsupportedDialect:
		// Recorded separately from "relayed" because nothing here can fix it:
		// the schema is affected and beyond this package's reach, so the answer
		// is a dialect to add or a backend to talk to, not a schema to inspect.
		relayed.record(field, "the declared dialect is not one the gateway translates")
		telemetry.RecordToolSchemaDialect(ctx, serverName, field, "unsupported_dialect")
	case result.Changed:
		telemetry.RecordToolSchemaDialect(ctx, serverName, field, "translated")
	}
	return normalized
}

// relayedDialects collects the schemas one server's discovery pass could not
// translate, so the reasons are reported once for the server rather than once
// per tool. A server with a hundred affected tools would otherwise write two
// hundred lines on every capability refresh, where every neighbouring
// diagnostic in listCapabilities is per server.
type relayedDialects struct {
	counts map[string]int
}

func newRelayedDialects() *relayedDialects {
	return &relayedDialects{counts: map[string]int{}}
}

func (r *relayedDialects) record(field, reason string) {
	if r == nil {
		return
	}
	r.counts[field+": "+reason]++
}

// report writes one line per distinct reason. Called after a server's tools are
// registered, from the same goroutine that recorded them.
func (r *relayedDialects) report(serverName string) {
	if r == nil || len(r.counts) == 0 {
		return
	}
	reasons := make([]string, 0, len(r.counts))
	for reason := range r.counts {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	for _, reason := range reasons {
		log.Logf("  > Relaying %d schema(s) from %s with the dialect declared, %s",
			r.counts[reason], serverName, reason)
	}
}
