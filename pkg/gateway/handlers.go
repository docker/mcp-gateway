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
		return g.clientPool.runToolContainer(ctx, tool, req.Params)
	}
}

func (g *Gateway) mcpServerToolHandler(serverConfig *catalog.ServerConfig, server *mcp.Server, annotations *mcp.ToolAnnotations) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		// Execute the tool call
		result, err := client.Session().CallTool(ctx, req.Params)

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

func (g *Gateway) mcpServerPromptHandler(serverConfig *catalog.ServerConfig, server *mcp.Server) mcp.PromptHandler {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
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

func (g *Gateway) mcpServerResourceHandler(serverConfig *catalog.ServerConfig, server *mcp.Server) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
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
