package interceptors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

// GitHubUnauthorizedMiddleware creates middleware that intercepts 401 unauthorized responses
// from the GitHub MCP server and returns the OAuth authorization link
func GitHubUnauthorizedMiddleware() mcp.Middleware[*mcp.ServerSession] {
	return func(next mcp.MethodHandler[*mcp.ServerSession]) mcp.MethodHandler[*mcp.ServerSession] {
		return func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			// Only intercept tools/call method
			if method != "tools/call" {
				return next(ctx, session, method, params)
			}

			// Check if this is a GitHub tool call
			var toolParams mcp.CallToolParams
			if paramsBytes, err := json.Marshal(params); err == nil {
				if err := json.Unmarshal(paramsBytes, &toolParams); err == nil {
					// Check if this is a GitHub tool (tools from GitHub server typically start with "github_")
					if !strings.HasPrefix(toolParams.Name, "github_") {
						return next(ctx, session, method, params)
					}
				}
			}

			// Call the actual handler
			response, err := next(ctx, session, method, params)

			// Check if we got a 401 unauthorized error
			if err != nil {
				errStr := err.Error()
				// Check for 401 or unauthorized in the error message
				if strings.Contains(strings.ToLower(errStr), "401") ||
					strings.Contains(strings.ToLower(errStr), "unauthorized") ||
					strings.Contains(strings.ToLower(errStr), "authentication required") {
					// Generate the OAuth URL
					authURL := generateGitHubOAuthURL()

					// Return a helpful message with the OAuth URL
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{
								Text: fmt.Sprintf(
									"GitHub authentication required. Please authorize access by running:\n\n"+
										"docker mcp oauth authorize github --scopes=repo\n\n"+
										"Or visit: %s",
									authURL,
								),
							},
						},
					}, nil
				}
			}

			// Check if the response itself indicates unauthorized
			if response != nil {
				if toolResult, ok := response.(*mcp.CallToolResult); ok && len(toolResult.Content) > 0 {
					for _, content := range toolResult.Content {
						if textContent, ok := content.(*mcp.TextContent); ok {
							text := strings.ToLower(textContent.Text)
							if strings.Contains(text, "401") ||
								strings.Contains(text, "unauthorized") ||
								strings.Contains(text, "authentication required") {
								// Generate the OAuth URL
								authURL := generateGitHubOAuthURL()

								// Return a helpful message with the OAuth URL
								return &mcp.CallToolResult{
									Content: []mcp.Content{
										&mcp.TextContent{
											Text: fmt.Sprintf(
												"GitHub authentication required. Please authorize access by running:\n\n"+
													"docker mcp oauth authorize github --scopes=repo\n\n"+
													"Or visit: %s",
												authURL,
											),
										},
									},
								}, nil
							}
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

