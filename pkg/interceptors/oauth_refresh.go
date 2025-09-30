package interceptors

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/contextkeys"
	"github.com/docker/mcp-gateway/pkg/oauth"
)

// OAuthRefreshMiddleware creates an interceptor that proactively checks and refreshes OAuth tokens
// before tool execution. This prevents tool calls from failing with 401 errors due to expired tokens.
//
// The middleware:
// 1. Only intercepts tools/call requests
// 2. Checks if the server has OAuth configuration
// 3. Verifies token validity and triggers refresh if needed (expires within 60s)
// 4. Coordinates multiple concurrent requests using singleflight pattern
// 5. Waits for both token storage AND server config reload before proceeding
func OAuthRefreshMiddleware(coordinator *oauth.RefreshCoordinator) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			// Only intercept tool calls
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			// Extract serverConfig from context (set by handler)
			serverConfig, ok := ctx.Value(contextkeys.ServerConfigKey).(*catalog.ServerConfig)
			if !ok || serverConfig == nil {
				// No server config in context - this is unexpected but not fatal
				// Proceed without OAuth check
				return next(ctx, method, req)
			}

			// Check if server has OAuth configuration
			if serverConfig.Spec.OAuth == nil || len(serverConfig.Spec.OAuth.Providers) == 0 {
				// No OAuth configured - fast path
				return next(ctx, method, req)
			}

			// Ensure token is valid before proceeding
			if err := coordinator.EnsureValidToken(ctx, serverConfig.Name); err != nil {
				// Token refresh failed - return error result to client
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{
							Text: fmt.Sprintf("OAuth token validation failed for %s: %v. Please run 'docker mcp oauth authorize %s' to authenticate.", serverConfig.Name, err, serverConfig.Name),
						},
					},
					IsError: true,
				}, nil
			}

			// Token is valid - proceed with tool execution
			return next(ctx, method, req)
		}
	}
}
