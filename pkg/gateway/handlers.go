package gateway

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

func getClientConfig(readOnlyHint *bool, ss *mcp.ServerSession, server *mcp.Server) *clientConfig {
	return &clientConfig{readOnly: readOnlyHint, serverSession: ss, server: server}
}

// inferServerType determines the type of MCP server based on its configuration
func inferServerType(serverConfig *catalog.ServerConfig) string {
	if serverConfig.Spec.Remote.Transport == "http" {
		return "streaming"
	}

	if serverConfig.Spec.Remote.Transport == "sse" {
		return "sse"
	}

	// Check for Docker image
	if serverConfig.Spec.Image != "" {
		return "docker"
	}

	// Unknown type
	return "unknown"
}

func (g *Gateway) mcpToolHandler(tool catalog.Tool) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Convert CallToolParamsRaw to CallToolParams.
		//
		// Arguments are forwarded as raw JSON (json.RawMessage) and intentionally
		// not unmarshaled here. The gateway must remain schema-agnostic and avoid
		// coercing tool inputs, preserving full argument fidelity for tools that
		// rely on structured or typed inputs.
		params := &mcp.CallToolParams{
			Meta: req.Params.Meta,
			Name: req.Params.Name,
		}

		// Forward raw arguments unchanged, if present.
		if len(req.Params.Arguments) > 0 {
			params.Arguments = req.Params.Arguments
		}

		return g.clientPool.runToolContainer(ctx, tool, params)
	}
}

func (g *Gateway) mcpServerToolHandler(serverName string, server *mcp.Server, annotations *mcp.ToolAnnotations, originalToolName string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Look up server configuration
		serverConfig, _, ok := g.configuration.Find(serverName)
		if !ok {
			return nil, fmt.Errorf("server %q not found in configuration", serverName)
		}

		// Debug logging to stderr
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-HANDLER] Tool call received: %s from server: %s\n", req.Params.Name, serverConfig.Name)
		}

		// Start telemetry span for tool call
		startTime := time.Now()
		serverType := inferServerType(serverConfig)

		// Build span attributes
		spanAttrs := []attribute.KeyValue{
			attribute.String("mcp.server.name", serverConfig.Name),
			attribute.String("mcp.server.type", serverType),
		}

		// Add additional server-specific attributes
		if serverConfig.Spec.Image != "" {
			spanAttrs = append(spanAttrs, attribute.String("mcp.server.image", serverConfig.Spec.Image))
		}
		if serverConfig.Spec.SSEEndpoint != "" {
			spanAttrs = append(spanAttrs, attribute.String("mcp.server.endpoint", serverConfig.Spec.SSEEndpoint))
		} else if serverConfig.Spec.Remote.URL != "" {
			spanAttrs = append(spanAttrs, attribute.String("mcp.server.endpoint", serverConfig.Spec.Remote.URL))
		}

		ctx, span := telemetry.StartToolCallSpan(ctx, req.Params.Name, spanAttrs...)
		defer span.End()

		// Record tool call counter with server attribution
		telemetry.ToolCallCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("mcp.server.name", serverConfig.Name),
				attribute.String("mcp.server.type", serverType),
				attribute.String("mcp.tool.name", req.Params.Name),
				attribute.String("mcp.client.name", req.Session.InitializeParams().ClientInfo.Name),
			),
		)

		var readOnlyHint *bool
		if annotations != nil && annotations.ReadOnlyHint {
			readOnlyHint = &annotations.ReadOnlyHint
		}

		client, err := g.clientPool.AcquireClient(ctx, serverConfig, getClientConfig(readOnlyHint, req.Session, server))
		if err != nil {
			// Record error in telemetry
			telemetry.RecordToolError(ctx, span, serverConfig.Name, serverType, req.Params.Name)
			span.SetStatus(codes.Error, "Failed to acquire client")
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		// Convert CallToolParamsRaw to CallToolParams.
		//
		// NOTE: Arguments are forwarded as raw JSON (json.RawMessage) instead of being
		// unmarshaled here. The gateway must not interpret or coerce tool arguments,
		// as it does not own the tool schema. Preserving the raw payload ensures full
		// fidelity for schema-based and typed tools and matches the MCP Go SDK
		// expectations.
		params := &mcp.CallToolParams{
			Meta: req.Params.Meta,
			Name: originalToolName,
		}

		// Forward raw arguments unchanged, if present.
		if len(req.Params.Arguments) > 0 {
			params.Arguments = req.Params.Arguments
		}

		// Execute the tool call
		result, err := client.Session().CallTool(ctx, params)

		// Record duration
		duration := time.Since(startTime).Milliseconds()
		telemetry.ToolCallDuration.Record(ctx, float64(duration),
			metric.WithAttributes(
				attribute.String("mcp.server.name", serverConfig.Name),
				attribute.String("mcp.server.type", serverType),
				attribute.String("mcp.tool.name", req.Params.Name),
				attribute.String("mcp.client.name", req.Session.InitializeParams().ClientInfo.Name),
			),
		)

		if err != nil {
			// Record error in telemetry
			telemetry.RecordToolError(ctx, span, serverConfig.Name, serverType, req.Params.Name)
			span.SetStatus(codes.Error, "Tool execution failed")
			return nil, err
		}

		span.SetStatus(codes.Ok, "")
		return result, nil
	}
}

func (g *Gateway) mcpServerPromptHandler(serverName string, server *mcp.Server) mcp.PromptHandler {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// Look up server configuration
		serverConfig, _, ok := g.configuration.Find(serverName)
		if !ok {
			return nil, fmt.Errorf("server %q not found in configuration", serverName)
		}

		// Debug logging to stderr
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-HANDLER] Prompt get received: %s from server: %s\n", req.Params.Name, serverConfig.Name)
		}

		// Start telemetry span for prompt operation
		startTime := time.Now()
		serverType := inferServerType(serverConfig)

		// Build span attributes
		spanAttrs := []attribute.KeyValue{
			attribute.String("mcp.server.name", serverConfig.Name),
			attribute.String("mcp.server.type", serverType),
		}

		// Add additional server-specific attributes
		if serverConfig.Spec.Image != "" {
			spanAttrs = append(spanAttrs, attribute.String("mcp.server.image", serverConfig.Spec.Image))
		}
		if serverConfig.Spec.SSEEndpoint != "" {
			spanAttrs = append(spanAttrs, attribute.String("mcp.server.endpoint", serverConfig.Spec.SSEEndpoint))
		} else if serverConfig.Spec.Remote.URL != "" {
			spanAttrs = append(spanAttrs, attribute.String("mcp.server.endpoint", serverConfig.Spec.Remote.URL))
		}

		ctx, span := telemetry.StartPromptSpan(ctx, req.Params.Name, spanAttrs...)
		defer span.End()

		// Record prompt get counter
		telemetry.RecordPromptGet(ctx, req.Params.Name, serverConfig.Name, req.Session.InitializeParams().ClientInfo.Name)

		client, err := g.clientPool.AcquireClient(ctx, serverConfig, getClientConfig(nil, req.Session, server))
		if err != nil {
			span.RecordError(err)
			telemetry.RecordPromptError(ctx, req.Params.Name, serverConfig.Name, "acquire_failed")
			span.SetStatus(codes.Error, "Failed to acquire client")
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		result, err := client.Session().GetPrompt(ctx, req.Params)

		// Record duration
		duration := time.Since(startTime).Milliseconds()
		telemetry.RecordPromptDuration(ctx, req.Params.Name, serverConfig.Name, float64(duration), req.Session.InitializeParams().ClientInfo.Name)

		if err != nil {
			span.RecordError(err)
			telemetry.RecordPromptError(ctx, req.Params.Name, serverConfig.Name, "execution_failed")
			span.SetStatus(codes.Error, "Prompt execution failed")
			return nil, err
		}

		span.SetStatus(codes.Ok, "")
		return result, nil
	}
}

func (g *Gateway) mcpServerResourceHandler(serverName string, server *mcp.Server) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		// Look up server configuration
		serverConfig, _, ok := g.configuration.Find(serverName)
		if !ok {
			return nil, fmt.Errorf("server %q not found in configuration", serverName)
		}

		// Debug logging to stderr
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-HANDLER] Resource read received: %s from server: %s\n", req.Params.URI, serverConfig.Name)
		}

		// Start telemetry span for resource operation
		startTime := time.Now()
		serverType := inferServerType(serverConfig)

		// Build span attributes - include server-specific attributes
		spanAttrs := []attribute.KeyValue{
			attribute.String("mcp.server.origin", serverConfig.Name),
			attribute.String("mcp.server.type", serverType),
		}

		// Add additional server-specific attributes
		if serverConfig.Spec.Image != "" {
			spanAttrs = append(spanAttrs, attribute.String("mcp.server.image", serverConfig.Spec.Image))
		}
		if serverConfig.Spec.SSEEndpoint != "" {
			spanAttrs = append(spanAttrs, attribute.String("mcp.server.endpoint", serverConfig.Spec.SSEEndpoint))
		}
		if serverConfig.Spec.Remote.URL != "" {
			spanAttrs = append(spanAttrs, attribute.String("mcp.server.remote_url", serverConfig.Spec.Remote.URL))
		}

		ctx, span := telemetry.StartResourceSpan(ctx, req.Params.URI, spanAttrs...)
		defer span.End()

		// Record counter with server attribution
		telemetry.RecordResourceRead(ctx, req.Params.URI, serverConfig.Name, req.Session.InitializeParams().ClientInfo.Name)

		client, err := g.clientPool.AcquireClient(ctx, serverConfig, getClientConfig(nil, req.Session, server))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to acquire client")
			telemetry.RecordResourceError(ctx, req.Params.URI, serverConfig.Name, "acquire_failed")
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		result, err := client.Session().ReadResource(ctx, req.Params)

		// Record duration regardless of error
		duration := time.Since(startTime).Milliseconds()
		telemetry.RecordResourceDuration(ctx, req.Params.URI, serverConfig.Name, float64(duration), req.Session.InitializeParams().ClientInfo.Name)

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Resource read failed")
			telemetry.RecordResourceError(ctx, req.Params.URI, serverConfig.Name, "read_failed")
			return nil, err
		}

		// Success
		span.SetStatus(codes.Ok, "")
		return result, nil
	}
}
