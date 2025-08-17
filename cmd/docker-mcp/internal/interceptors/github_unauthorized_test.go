package interceptors

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAuthenticationError(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "401 with github.com",
			text:     "Error: 401 Unauthorized from api.github.com",
			expected: true,
		},
		{
			name:     "401 with Bad credentials",
			text:     "HTTP 401: Bad credentials",
			expected: true,
		},
		{
			name:     "401 with Unauthorized",
			text:     "Request failed with 401 Unauthorized",
			expected: true,
		},
		{
			name:     "Bad credentials without 401",
			text:     "Authentication error: Bad credentials",
			expected: false,
		},
		{
			name:     "401 without GitHub indicators",
			text:     "HTTP 401 from some-other-service.com",
			expected: false,
		},
		{
			name:     "403 Forbidden with github.com",
			text:     "Error: 403 Forbidden from api.github.com",
			expected: false,
		},
		{
			name:     "wrapped error with 401 and Bad credentials",
			text:     `calling "tools/call": HTTP 401: Bad credentials`,
			expected: true,
		},
		{
			name:     "complex GitHub API error",
			text:     "Error calling GitHub API: 401 Unauthorized - Bad credentials",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAuthenticationError(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGitHubUnauthorizedMiddleware(t *testing.T) {
	t.Run("ignores non-tools-call methods", func(t *testing.T) {
		mockHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return &mcp.ListResourcesResult{}, nil
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "resources/list", nil)

		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("passes through errors unchanged", func(t *testing.T) {
		expectedErr := errors.New("some network error")
		mockHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return nil, expectedErr
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, result)
	})

	t.Run("passes through successful responses", func(t *testing.T) {
		expectedResult := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Success!"},
			},
		}
		mockHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return expectedResult, nil
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("passes through error responses without auth failure", func(t *testing.T) {
		expectedResult := &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Some other error"},
			},
		}
		mockHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return expectedResult, nil
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("intercepts exact GitHub auth error", func(t *testing.T) {
		mockHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "HTTP 401: Bad credentials"},
				},
			}, nil
		}

		mockOAuth := func(_ context.Context) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "GitHub authentication required. Please authorize at:\nhttps://mock-oauth-url.com\n\nNote: After authorizing, retry your request."},
				},
			}, nil
		}

		middleware := GitHubUnauthorizedMiddlewareWithOAuth(mockOAuth)
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		require.NoError(t, err)
		require.NotNil(t, result)

		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		require.Len(t, toolResult.Content, 1)

		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "GitHub authentication required")
	})

	t.Run("intercepts wrapped GitHub auth error", func(t *testing.T) {
		mockHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: `calling "tools/call": HTTP 401: Bad credentials`},
				},
			}, nil
		}

		mockOAuth := func(_ context.Context) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "GitHub authentication required. Please authorize at:\nhttps://mock-oauth-url.com\n\nNote: After authorizing, retry your request."},
				},
			}, nil
		}

		middleware := GitHubUnauthorizedMiddlewareWithOAuth(mockOAuth)
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		require.NoError(t, err)
		require.NotNil(t, result)

		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		require.Len(t, toolResult.Content, 1)

		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "GitHub authentication required")
	})

	t.Run("intercepts auth error among multiple content items", func(t *testing.T) {
		mockHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "First message"},
					&mcp.TextContent{Text: "HTTP 401: Bad credentials"},
					&mcp.TextContent{Text: "Third message"},
				},
			}, nil
		}

		mockOAuth := func(_ context.Context) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "GitHub authentication required. Please authorize at:\nhttps://mock-oauth-url.com\n\nNote: After authorizing, retry your request."},
				},
			}, nil
		}

		middleware := GitHubUnauthorizedMiddlewareWithOAuth(mockOAuth)
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		require.NoError(t, err)
		require.NotNil(t, result)

		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		require.Len(t, toolResult.Content, 1)

		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "GitHub authentication required")
	})

	t.Run("handles non-text content gracefully", func(t *testing.T) {
		mockHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.ImageContent{Data: []byte("base64data"), MIMEType: "image/png"},
					&mcp.TextContent{Text: "HTTP 401: Bad credentials"},
				},
			}, nil
		}

		mockOAuth := func(_ context.Context) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "GitHub authentication required. Please authorize at:\nhttps://mock-oauth-url.com\n\nNote: After authorizing, retry your request."},
				},
			}, nil
		}

		middleware := GitHubUnauthorizedMiddlewareWithOAuth(mockOAuth)
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		require.NoError(t, err)
		require.NotNil(t, result)

		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		require.Len(t, toolResult.Content, 1)

		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "GitHub authentication required")
	})

	t.Run("handles OAuth flow errors gracefully", func(t *testing.T) {
		mockHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "HTTP 401: Bad credentials"},
				},
			}, nil
		}

		mockOAuth := func(_ context.Context) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Failed to get GitHub OAuth URL: connection refused"},
				},
				IsError: true,
			}, nil
		}

		middleware := GitHubUnauthorizedMiddlewareWithOAuth(mockOAuth)
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		require.NoError(t, err)
		require.NotNil(t, result)

		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		require.Len(t, toolResult.Content, 1)
		assert.True(t, toolResult.IsError)

		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "Failed to get GitHub OAuth URL")
	})
}
