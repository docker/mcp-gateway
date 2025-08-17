package interceptors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

// isAuthenticationError checks if a text contains authentication-related error messages
func isAuthenticationError(text string) bool {
	// Check for any 401 error from GitHub API
	return strings.Contains(text, "401") &&
		(strings.Contains(text, "github.com") ||
			strings.Contains(text, "Bad credentials") ||
			strings.Contains(text, "Unauthorized"))
}

// OAuthHandler defines the interface for handling OAuth flows
type OAuthHandler func(ctx context.Context) (*mcp.CallToolResult, error)

// GitHubUnauthorizedMiddleware creates middleware that intercepts 401 unauthorized responses
// from the GitHub MCP server and returns the OAuth authorization link
func GitHubUnauthorizedMiddleware() mcp.Middleware[*mcp.ServerSession] {
	return GitHubUnauthorizedMiddlewareWithOAuth(handleOAuthFlow)
}

// GitHubUnauthorizedMiddlewareWithOAuth creates middleware with a configurable OAuth handler for testing
func GitHubUnauthorizedMiddlewareWithOAuth(oauthHandler OAuthHandler) mcp.Middleware[*mcp.ServerSession] {
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
					// Start OAuth flow and wait for completion
					return oauthHandler(ctx)
				}
			}

			return response, err
		}
	}
}

// handleOAuthFlow manages the simplified OAuth flow
func handleOAuthFlow(_ context.Context) (*mcp.CallToolResult, error) {
	// Get OAuth URL without opening browser
	authURL, err := getGitHubOAuthURL()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Failed to get GitHub OAuth URL: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	fmt.Printf("OAuth URL generated: %s\n", authURL)

	// Return the auth URL for the user - Docker Desktop will handle the callback
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("GitHub authentication required. Please authorize at:\n%s\n\nNote: After authorizing, retry your request.", authURL),
			},
		},
	}, nil
}

// getGitHubOAuthURL gets the OAuth URL using the MCP Gateway endpoint (no browser opening)
func getGitHubOAuthURL() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the MCP Gateway specific endpoint that doesn't open browser
	client := desktop.NewAuthClient()
	authResponse, err := client.PostOAuthAppMCPGateway(ctx, "github", "repo read:packages read:user")
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth URL from Docker Desktop: %w", err)
	}

	return authResponse.BrowserURL, nil
}
