package interceptors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/secretsscan"
)

func BlockSecretsMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			// Only check secrets for tools/call method
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			var toolName string
			var arguments any

			// Try to extract from request
			if callReq, ok := req.(*mcp.CallToolRequest); ok && callReq.Params != nil {
				toolName = callReq.Params.Name
				arguments = callReq.Params.Arguments
			}

			if toolName != "" {
				log.Logf("  - Scanning tool call arguments for secrets...\n")

				argumentsStr := argumentsToString(arguments)
				if secretsscan.ContainsSecrets(argumentsStr) {
					return nil, fmt.Errorf("a secret is being passed to tool %s", toolName)
				}

				log.Logf("  > No secret found in arguments.\n")
			}

			result, err := next(ctx, method, req)
			if err != nil {
				return result, err
			}

			// Check response for secrets
			if result != nil {
				log.Logf("  - Scanning tool call response for secrets...\n")

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

				log.Logf("  > No secret found in response.\n")
			}

			return result, nil
		}
	}
}
