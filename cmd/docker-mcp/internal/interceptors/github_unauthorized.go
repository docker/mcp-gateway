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
	openGitHubOAuthURL()
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "GitHub authentication required. Tell the user to complete GitHub authorization and stop trying to process this user's request as they haven't yet logged in.",
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

			// Call the actual handler
			response, err := next(ctx, session, method, params)

			// Pass through any actual errors
			if err != nil {
				return response, err
			}

			// Check if the response contains a GitHub authentication error
			toolResult, ok := response.(*mcp.CallToolResult)
			if !ok || !toolResult.IsError || len(toolResult.Content) == 0 {
				return response, err
			}

			// Check each content item for the authentication error message
			for _, content := range toolResult.Content {
				textContent, ok := content.(*mcp.TextContent)
				if !ok {
					continue
				}
				if isAuthenticationError(textContent.Text) {
					return createAuthRequiredResponse(), nil
				}
			}

			return response, err
		}
	}
}

// openGitHubOAuthURL triggers the browser to open the GitHub OAuth authorization page
func openGitHubOAuthURL() {
	// This simulates what happens when running "docker mcp oauth authorize github --scopes=repo"
	// In a real implementation, we'd call the desktop API to get the actual URL
	ctx := context.Background()
	client := desktop.NewAuthClient()

	_, err := client.PostOAuthApp(ctx, "github", "repo")
	if err != nil {
		// Log error but continue - browser should still open
		fmt.Printf("Error opening OAuth URL: %v\n", err)
		return
	}
}
