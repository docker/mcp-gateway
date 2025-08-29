package interceptors

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func LogCallsMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			// Only log tools/call method
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			start := time.Now()

			// Extract tool name from request
			var toolName string
			var arguments any

			// Try to extract from request
			if callReq, ok := req.(*mcp.CallToolRequest); ok && callReq.Params != nil {
				toolName = callReq.Params.Name
				arguments = callReq.Params.Arguments
			}

			if toolName != "" {
				logf("  - Calling tool %s with arguments: %s\n", toolName, argumentsToString(arguments))
			} else {
				logf("  - Calling tool (unknown) with method: %s\n", method)
			}

			result, err := next(ctx, method, req)
			if err != nil {
				return result, err
			}

			logf("  > Calling tool %s took: %s\n", toolName, time.Since(start))

			return result, nil
		}
	}
}
