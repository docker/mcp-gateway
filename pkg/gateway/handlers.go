package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/policy"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

func getClientConfig(ss *mcp.ServerSession, server *mcp.Server) *clientConfig {
	return &clientConfig{serverSession: ss, server: server}
}

// inferServerTransportType determines a transport type label for telemetry.
// It returns streaming, sse, docker, or unknown based on server configuration.
func inferServerTransportType(serverConfig *catalog.ServerConfig) string {
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

func (g *Gateway) pociToolHandler(tool catalog.Tool) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args any
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
			}
		}
		params := &mcp.CallToolParams{
			Meta:      req.Params.Meta,
			Name:      req.Params.Name,
			Arguments: args,
		}
		return g.clientPool.runToolContainer(ctx, tool, params), nil
	}
}

func (g *Gateway) withPOCIToolTelemetry(serverName string, tool catalog.Tool, next mcp.ToolHandler) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		clientName := ""
		if clientInfo := auditClientInfoFromSession(req.Session); clientInfo != nil {
			clientName = clientInfo.Name
		}
		startTime := time.Now()
		spanAttrs := []attribute.KeyValue{
			attribute.String("mcp.server.name", serverName),
			attribute.String("mcp.server.type", "docker"),
			attribute.String("mcp.server.image", tool.Container.Image),
		}
		ctx, span := telemetry.StartToolCallSpan(ctx, req.Params.Name, spanAttrs...)
		metricAttrs := []attribute.KeyValue{
			attribute.String("mcp.server.name", serverName),
			attribute.String("mcp.server.type", "docker"),
			attribute.String("mcp.tool.name", req.Params.Name),
			attribute.String("mcp.client.name", clientName),
		}
		defer func() {
			telemetry.ToolCallDuration.Record(ctx, float64(time.Since(startTime).Milliseconds()), metric.WithAttributes(metricAttrs...))
			span.End()
		}()
		telemetry.ToolCallCounter.Add(ctx, 1, metric.WithAttributes(metricAttrs...))

		result, err := next(ctx, req)
		if err != nil {
			telemetry.RecordToolError(ctx, span, serverName, "docker", req.Params.Name)
			span.SetStatus(codes.Error, "Tool call failed")
			return nil, err
		}
		if result != nil && result.IsError {
			telemetry.RecordToolError(ctx, span, serverName, "docker", req.Params.Name)
			span.SetStatus(codes.Error, "Tool execution failed")
			return result, nil
		}
		span.SetStatus(codes.Ok, "")
		return result, nil
	}
}

func (g *Gateway) mcpServerToolHandler(serverName string, server *mcp.Server, _ *mcp.ToolAnnotations, originalToolName string) mcp.ToolHandler {
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

		client, err := g.clientPool.AcquireClient(ctx, serverConfig, getClientConfig(req.Session, server))
		if err != nil {
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		// Convert CallToolParamsRaw to CallToolParams
		var args any
		if len(req.Params.Arguments) > 0 {
			if jsonErr := json.Unmarshal(req.Params.Arguments, &args); jsonErr != nil {
				return nil, fmt.Errorf("failed to unmarshal arguments: %w", jsonErr)
			}
		}
		params := &mcp.CallToolParams{
			Meta:      req.Params.Meta,
			Name:      originalToolName,
			Arguments: args,
		}

		// Execute the tool call
		result, err := client.Session().CallTool(ctx, params)
		if err != nil {
			return nil, err
		}

		return result, nil
	}
}

func (g *Gateway) mcpServerPromptHandler(serverName string, server *mcp.Server, originalPromptName string) mcp.PromptHandler {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// Look up server configuration
		serverConfig, _, ok := g.configuration.Find(serverName)
		if !ok {
			return nil, fmt.Errorf("server %q not found in configuration", serverName)
		}

		if g.policyClient != nil {
			policyReq := g.configuration.policyRequest(
				serverConfig.Name,
				originalPromptName,
				policy.ActionPrompt,
			)
			decision, err := g.policyClient.Evaluate(ctx, policyReq)
			event := buildAuditEvent(policyReq, decision, err, auditClientInfoFromSession(req.Session))
			submitAuditEvent(g.policyClient, event)
			if err != nil {
				return nil, fmt.Errorf("policy check failed for prompt %s on server %s: %w", originalPromptName, serverConfig.Name, err)
			}
			if !decision.Allowed {
				return nil, fmt.Errorf("policy denied prompt %s on server %s: %s", originalPromptName, serverConfig.Name, decision.Reason)
			}
		}

		// Debug logging to stderr
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-HANDLER] Prompt get received: %s from server: %s\n", req.Params.Name, serverConfig.Name)
		}

		// Start telemetry span for prompt operation
		startTime := time.Now()
		serverTransportType := inferServerTransportType(serverConfig)

		// Build span attributes
		spanAttrs := []attribute.KeyValue{
			attribute.String("mcp.server.name", serverConfig.Name),
			attribute.String("mcp.server.type", serverTransportType),
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

		client, err := g.clientPool.AcquireClient(ctx, serverConfig, getClientConfig(req.Session, server))
		if err != nil {
			span.RecordError(err)
			telemetry.RecordPromptError(ctx, req.Params.Name, serverConfig.Name, "acquire_failed")
			span.SetStatus(codes.Error, "Failed to acquire client")
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		params := *req.Params
		params.Name = originalPromptName
		result, err := client.Session().GetPrompt(ctx, &params)

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
		serverTransportType := inferServerTransportType(serverConfig)

		// Build span attributes - include server-specific attributes
		spanAttrs := []attribute.KeyValue{
			attribute.String("mcp.server.origin", serverConfig.Name),
			attribute.String("mcp.server.type", serverTransportType),
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

		client, err := g.clientPool.AcquireClient(ctx, serverConfig, getClientConfig(req.Session, server))
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
