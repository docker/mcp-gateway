package interceptors

import (
	"context"
	"encoding/json"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func LogCallsMiddleware() mcp.Middleware[*mcp.ServerSession] {
	return func(next mcp.MethodHandler[*mcp.ServerSession]) mcp.MethodHandler[*mcp.ServerSession] {
		return func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			// Only log tools/call method
			if method != "tools/call" {
				return next(ctx, session, method, params)
			}

			start := time.Now()

			// Extract tool name from params by marshaling/unmarshaling
			var toolName string
			var arguments any

			// Try to extract from JSON
			if jsonData, err := json.Marshal(params); err == nil {
				var callParams mcp.CallToolParams
				if err := json.Unmarshal(jsonData, &callParams); err == nil {
					toolName = callParams.Name
					arguments = callParams.Arguments
				}
			}

			if toolName != "" {
				logf("  - Calling tool %s with arguments: %s\n", toolName, argumentsToString(arguments))
			} else {
				logf("  - Calling tool (unknown) with method: %s\n", method)
			}

			result, err := next(ctx, session, method, params)
			if err != nil {
				return result, err
			}

			logf("  > Calling tool %s took: %s\n", toolName, time.Since(start))
			logf("  > Calling tool test message %s took: %s\n", toolName, time.Since(start))

			return result, nil
		}
	}
}
