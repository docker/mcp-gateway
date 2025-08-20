package interceptors

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/contextkeys"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

// getGitHubOAuthURL is a variable function so it can be mocked in tests
var getGitHubOAuthURL = getGitHubOAuthURLImpl

// isAuthenticationError checks if a text contains GitHub authentication-related error messages
func isAuthenticationError(text string) bool {
	// Must contain 401 status code
	if !strings.Contains(text, "401") {
		return false
	}

	// Must contain GitHub-specific indicators
	return strings.Contains(text, "github.com") ||
		strings.Contains(text, "api.github.com") ||
		(strings.Contains(text, "Bad credentials") &&
			(strings.Contains(text, "github") || strings.Contains(text, "GitHub")))
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

	fmt.Fprintf(os.Stderr, "OAuth URL generated: %s\n", authURL)

	// Return the auth URL for the user - Docker Desktop will handle the callback
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("GitHub authentication required. Please authorize at:\n%s\n\nNote: After authorizing, retry your request.", authURL),
			},
		},
	}, nil
}

// getGitHubOAuthURLImpl gets the OAuth URL with auto-open disabled from Docker Desktop
func getGitHubOAuthURLImpl() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Add feature flag to context - true because this is only called when oauth-interceptor feature is enabled
	ctx = context.WithValue(ctx, contextkeys.OAuthInterceptorEnabledKey, true)

	// Use PostOAuthApp with disableAutoOpen: true to prevent automatic browser opening
	client := desktop.NewAuthClient()
	authResponse, err := client.PostOAuthApp(ctx, "github", "repo read:packages read:user", true)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth URL from Docker Desktop: %w", err)
	}

	return authResponse.BrowserURL, nil
}
