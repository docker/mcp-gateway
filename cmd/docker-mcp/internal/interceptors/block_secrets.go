package interceptors

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/secretsscan"
)

func BlockSecrets(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logf("  - Scanning tool call arguments for secrets...\n")

		arguments := argumentsToString(request.Params.Arguments)
		if secretsscan.ContainsSecrets(arguments) {
			return nil, fmt.Errorf("a secret is being passed to tool %s", request.Params.Name)
		}

		logf("  > No secret found in arguments.\n")

		result, err := next(ctx, request)
		if err != nil {
			return result, err
		}

		logf("  - Scanning tool call response for secrets...\n")

		var contents string
		for _, content := range result.Content {
			switch c := content.(type) {
			case mcp.TextContent:
				contents += c.Text
			case *mcp.TextContent:
				contents += c.Text
			}
		}

		if secretsscan.ContainsSecrets(contents) {
			return nil, fmt.Errorf("a secret is being returned by the %s tool", request.Params.Name)
		}

		logf("  > No secret found in response.\n")

		return result, nil
	}
}
