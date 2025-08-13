package interceptors

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubUnauthorizedMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		toolName        string
		handlerError    error
		handlerResult   mcp.Result
		expectIntercept bool
		expectError     bool
	}{
		{
			name:            "non-tools-call-method-passes-through",
			method:          "resources/list",
			toolName:        "github_list_repos",
			handlerResult:   &mcp.ListResourcesResult{},
			expectIntercept: false,
			expectError:     false,
		},
		{
			name:            "non-github-tool-passes-through",
			method:          "tools/call",
			toolName:        "other_tool",
			handlerResult:   &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "success"}}},
			expectIntercept: false,
			expectError:     false,
		},
		{
			name:            "github-tool-with-401-error-intercepted",
			method:          "tools/call",
			toolName:        "github_list_repos",
			handlerError:    errors.New("401 Unauthorized"),
			expectIntercept: true,
			expectError:     false,
		},
		{
			name:            "github-tool-with-unauthorized-error-intercepted",
			method:          "tools/call",
			toolName:        "github_create_issue",
			handlerError:    errors.New("Request failed: unauthorized"),
			expectIntercept: true,
			expectError:     false,
		},
		{
			name:            "github-tool-with-authentication-required-error-intercepted",
			method:          "tools/call",
			toolName:        "github_get_user",
			handlerError:    errors.New("Authentication required to access this resource"),
			expectIntercept: true,
			expectError:     false,
		},
		{
			name:     "github-tool-with-401-in-response-intercepted",
			method:   "tools/call",
			toolName: "github_list_repos",
			handlerResult: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Error: 401 Unauthorized - Please authenticate"},
				},
			},
			expectIntercept: true,
			expectError:     false,
		},
		{
			name:     "github-tool-with-unauthorized-in-response-intercepted",
			method:   "tools/call",
			toolName: "github_create_pr",
			handlerResult: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Request failed: Unauthorized access to repository"},
				},
			},
			expectIntercept: true,
			expectError:     false,
		},
		{
			name:     "github-tool-with-success-passes-through",
			method:   "tools/call",
			toolName: "github_list_repos",
			handlerResult: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Successfully listed repositories"},
				},
			},
			expectIntercept: false,
			expectError:     false,
		},
		{
			name:            "github-tool-with-other-error-passes-through",
			method:          "tools/call",
			toolName:        "github_list_repos",
			handlerError:    errors.New("Network timeout"),
			expectIntercept: false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock handler
			mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
				return tt.handlerResult, tt.handlerError
			}

			// Create the middleware
			middleware := GitHubUnauthorizedMiddleware()
			wrappedHandler := middleware(mockHandler)

			// Create test params
			params := &mcp.CallToolParams{
				Name: tt.toolName,
			}

			// Call the wrapped handler
			result, err := wrappedHandler(context.Background(), nil, tt.method, params)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectIntercept {
				// Check that we got an intercepted response
				require.NotNil(t, result)
				toolResult, ok := result.(*mcp.CallToolResult)
				require.True(t, ok, "Expected CallToolResult")
				require.Len(t, toolResult.Content, 1)

				textContent, ok := toolResult.Content[0].(*mcp.TextContent)
				require.True(t, ok, "Expected TextContent")

				// Check that the response contains OAuth information
				assert.Contains(t, textContent.Text, "GitHub authentication required")
				assert.Contains(t, textContent.Text, "docker mcp oauth authorize github")
			} else if tt.handlerResult != nil && !tt.expectError {
				// Check that the original result was returned
				assert.Equal(t, tt.handlerResult, result)
			}
		})
	}
}

func TestGitHubUnauthorizedMiddleware_NonGitHubTools(t *testing.T) {
	// Test that non-GitHub tools are not intercepted even with 401 errors
	mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
		return nil, errors.New("401 Unauthorized")
	}

	middleware := GitHubUnauthorizedMiddleware()
	wrappedHandler := middleware(mockHandler)

	// Test with a non-GitHub tool name
	params := &mcp.CallToolParams{
		Name: "docker_list_containers",
	}

	_, err := wrappedHandler(context.Background(), nil, "tools/call", params)

	// The error should pass through unchanged for non-GitHub tools
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401 Unauthorized")
}

func TestGitHubUnauthorizedMiddleware_MultipleContentItems(t *testing.T) {
	// Test handling of multiple content items in response
	mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "First message"},
				&mcp.TextContent{Text: "Error: 401 Unauthorized"},
				&mcp.TextContent{Text: "Third message"},
			},
		}, nil
	}

	middleware := GitHubUnauthorizedMiddleware()
	wrappedHandler := middleware(mockHandler)

	params := &mcp.CallToolParams{
		Name: "github_api_call",
	}

	result, err := wrappedHandler(context.Background(), nil, "tools/call", params)

	assert.NoError(t, err)
	require.NotNil(t, result)

	toolResult, ok := result.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, toolResult.Content, 1)

	textContent, ok := toolResult.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "GitHub authentication required")
}

func TestGitHubUnauthorizedMiddleware_CaseInsensitive(t *testing.T) {
	// Test that error detection is case-insensitive
	testCases := []string{
		"401 UNAUTHORIZED",
		"401 unauthorized",
		"Error: Unauthorized",
		"AUTHENTICATION REQUIRED",
		"Authentication Required",
	}

	for _, errorMsg := range testCases {
		t.Run(errorMsg, func(t *testing.T) {
			mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
				return nil, errors.New(errorMsg)
			}

			middleware := GitHubUnauthorizedMiddleware()
			wrappedHandler := middleware(mockHandler)

			params := &mcp.CallToolParams{
				Name: "github_test",
			}

			result, err := wrappedHandler(context.Background(), nil, "tools/call", params)

			assert.NoError(t, err)
			require.NotNil(t, result)

			toolResult, ok := result.(*mcp.CallToolResult)
			require.True(t, ok)

			textContent, ok := toolResult.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.Contains(t, textContent.Text, "GitHub authentication required")
		})
	}
}

