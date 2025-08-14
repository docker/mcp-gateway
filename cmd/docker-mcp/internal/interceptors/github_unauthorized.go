package interceptors

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/oauth"
)

// isAuthenticationError checks if a text contains authentication-related error messages
func isAuthenticationError(text string) bool {
	// Check for the exact error message from GitHub
	// The error comes wrapped as: calling "tools/call": Authentication Failed: Bad credentials
	return strings.Contains(text, "Authentication Failed: Bad credentials")
}

// createAuthRequiredResponse creates the standardized authentication required response
func createAuthRequiredResponse() *mcp.CallToolResult {
	// Start OAuth callback server on port 3278
	oauth.StartCallbackServer(3278)
	
	// Get OAuth URL without opening browser
	authURL, err := getGitHubOAuthURLWithoutBrowser()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Failed to get GitHub OAuth URL: %v", err),
				},
			},
			IsError: true,
		}
	}
	
	// Return the OAuth URL for the user to open
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("GitHub authentication required. Please authorize at: %s\n\nWaiting for authorization...", authURL),
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
					// Start OAuth flow and wait for completion
					return handleOAuthFlow(ctx)
				}
			}

			return response, err
		}
	}
}

// handleOAuthFlow manages the complete OAuth flow
func handleOAuthFlow(ctx context.Context) (*mcp.CallToolResult, error) {
	// Start OAuth callback server on port 3278
	oauth.StartCallbackServer(3278)
	defer oauth.StopCallbackServer()
	
	// Get OAuth URL without opening browser
	authURL, err := getGitHubOAuthURLWithoutBrowser()
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
	
	// Start a goroutine to complete the OAuth flow
	tokenChan := make(chan *oauth.GitHubToken, 1)
	errChan := make(chan error, 1)
	
	go func() {
		token, err := oauth.CompleteOAuthFlow(ctx)
		if err != nil {
			errChan <- err
			return
		}
		tokenChan <- token
	}()
	
	// Return immediately with the auth URL for the user
	// The actual token exchange will happen in the background
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("GitHub authentication required. Please authorize at:\n%s\n\nNote: After authorizing, retry your request.", authURL),
			},
		},
	}, nil
}

// getGitHubOAuthURLWithoutBrowser gets the OAuth URL without opening the browser
func getGitHubOAuthURLWithoutBrowser() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Check if we should use mock mode (for testing without replacing Docker Desktop)
	if os.Getenv("USE_MOCK_AUTH") == "true" {
		mockClient := desktop.NewMockAuthClient()
		authResponse, err := mockClient.PostOAuthAppMCPGateway(ctx, "github", "")
		if err != nil {
			return "", fmt.Errorf("failed to get OAuth URL from mock: %w", err)
		}
		return authResponse.BrowserURL, nil
	}
	
	// Try to use the new endpoint (only works if running modified Docker Desktop)
	client := desktop.NewAuthClient()
	authResponse, err := client.PostOAuthAppMCPGateway(ctx, "github", "")
	if err != nil {
		// Fallback to regular endpoint (will open browser but at least works)
		fmt.Println("Note: Using fallback mode - browser will open")
		authResponse, err = client.PostOAuthApp(ctx, "github", "")
		if err != nil {
			return "", fmt.Errorf("failed to get OAuth URL: %w", err)
		}
	}
	
	return authResponse.BrowserURL, nil
}
