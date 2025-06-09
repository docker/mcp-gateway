package interceptors

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func LogCalls(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()

		logf("- Calling tool %s with arguments: %s\n", request.Params.Name, argumentsToString(request.Params.Arguments))

		result, err := next(ctx, request)
		if err != nil {
			return result, err
		}

		logf("> Calling tool %s took: %s\n", request.Params.Name, time.Since(start))

		return result, nil
	}
}
