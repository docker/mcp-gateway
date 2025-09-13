package interceptors

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/telemetry"
)

// TelemetryMiddleware tracks list operations and other gateway operations
func TelemetryMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			// Debug log all methods if debug is enabled
			if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[MCP-MIDDLEWARE] Method called: %s\n", method)
			}

			// Track list operations with spans and metrics
			var span trace.Span
			var tracked bool

			switch method {
			case "initialize":
				params := req.GetParams().(*mcp.InitializeParams)
				ctx, span = telemetry.StartInitializeSpan(ctx)
				telemetry.RecordInitialize(ctx, params)
				tracked = true
			case "tools/list":
				ctx, span = telemetry.StartListSpan(ctx, "tools")
				telemetry.RecordListTools(ctx)
				tracked = true
			case "prompts/list":
				ctx, span = telemetry.StartListSpan(ctx, "prompts")
				telemetry.RecordListPrompts(ctx)
				tracked = true
			case "resources/list":
				ctx, span = telemetry.StartListSpan(ctx, "resources")
				telemetry.RecordListResources(ctx)
				tracked = true
			case "resourceTemplates/list":
				ctx, span = telemetry.StartListSpan(ctx, "resourceTemplates")
				telemetry.RecordListResourceTemplates(ctx)
				tracked = true
			}

			// Call the next handler
			result, err := next(ctx, method, req)

			// Complete the span if we created one
			if tracked && span != nil {
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, fmt.Sprintf("List %s failed", method))
				} else {
					span.SetStatus(codes.Ok, "")
				}
				span.End()
			}

			return result, err
		}
	}
}
