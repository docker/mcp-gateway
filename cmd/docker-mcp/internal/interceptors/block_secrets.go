package interceptors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/secretsscan"
)

func BlockSecretsMiddleware() mcp.Middleware[*mcp.ServerSession] {
	return func(next mcp.MethodHandler[*mcp.ServerSession]) mcp.MethodHandler[*mcp.ServerSession] {
		return func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			// Only check secrets for tools/call method
			if method != "tools/call" {
				return next(ctx, session, method, params)
			}

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
				logf("  - Scanning tool call arguments for secrets...\n")

				argumentsStr := argumentsToString(arguments)
				if secretsscan.ContainsSecrets(argumentsStr) {
					return nil, fmt.Errorf("a secret is being passed to tool %s", toolName)
				}

				logf("  > No secret found in arguments.\n")
			}

			result, err := next(ctx, session, method, params)
			if err != nil {
				return result, err
			}

			// Check response for secrets
			if result != nil {
				logf("  - Scanning tool call response for secrets...\n")

				var contents string

				// Try to extract content from JSON result
				if jsonData, err := json.Marshal(result); err == nil {
					var callResult mcp.CallToolResult
					if err := json.Unmarshal(jsonData, &callResult); err == nil {
						for _, content := range callResult.Content {
							if c, ok := content.(*mcp.TextContent); ok {
								contents += c.Text
							}
						}
					}
				}

				if contents != "" && secretsscan.ContainsSecrets(contents) {
					return nil, fmt.Errorf("a secret is being returned by the %s tool", toolName)
				}

				logf("  > No secret found in response.\n")
			}

			return result, nil
		}
	}
}
