package interceptors

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

// isAuthenticationError checks if a text contains authentication-related error messages
func isAuthenticationError(text string) bool {
	// Check for the exact error message from GitHub
	// The error comes wrapped as: calling "tools/call": Authentication Failed: Bad credentials
	return strings.Contains(text, "Authentication Failed: Bad credentials")
}

// createAuthRequiredResponse creates the standardized authentication required response
func createAuthRequiredResponse() *mcp.CallToolResult {
	authURL := generateGitHubOAuthURL()
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf(
					"GitHub authentication required. Use this link to authorize: %s",
					authURL,
				),
			},
		},
	}
}

// GitHubUnauthorizedMiddleware creates middleware that intercepts 401 unauthorized responses
// from the GitHub MCP server and returns the OAuth authorization link
func GitHubUnauthorizedMiddleware() mcp.Middleware[*mcp.ServerSession] {
	return func(next mcp.MethodHandler[*mcp.ServerSession]) mcp.MethodHandler[*mcp.ServerSession] {
		return func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			// Only intercept tools/call method
			if method != "tools/call" {
				return next(ctx, session, method, params)
			}

			// Don't check tool name - intercept all tool calls
			// The GitHub MCP server tools don't have "github_" prefix

			// Call the actual handler
			response, err := next(ctx, session, method, params)

			// Check if we got an unauthorized error
			if err != nil && isAuthenticationError(err.Error()) {
				return createAuthRequiredResponse(), nil
			}

			// If the tool call didn't return an explicit error, check the response for authentication failure message
			toolResult, ok := response.(*mcp.CallToolResult)
			if ok && toolResult != nil {
				if toolResult.IsError && len(toolResult.Content) > 0 {
					for _, content := range toolResult.Content {
						textContent, ok := content.(*mcp.TextContent)
						if !ok {
							continue
						}
						if isAuthenticationError(textContent.Text) {
							return createAuthRequiredResponse(), nil
						}
					}
				}
			}

			return response, err
		}
	}
}

// generateGitHubOAuthURL generates the OAuth URL for GitHub authorization
func generateGitHubOAuthURL() string {
	// This simulates what happens when running "docker mcp oauth authorize github --scopes=repo"
	// In a real implementation, we'd call the desktop API to get the actual URL
	ctx := context.Background()
	client := desktop.NewAuthClient()

	authResponse, err := client.PostOAuthApp(ctx, "github", "repo")
	if err != nil {
		// Fallback to a generic message if we can't get the URL
		return "GitHub OAuth authorization page"
	}

	return authResponse.BrowserURL
}
