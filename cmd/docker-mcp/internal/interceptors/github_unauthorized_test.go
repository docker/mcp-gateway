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
	t.Run("ignores non-tools-call methods", func(t *testing.T) {
		mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			return &mcp.ListResourcesResult{}, nil
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "resources/list", nil)
		
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("passes through errors unchanged", func(t *testing.T) {
		expectedErr := errors.New("some network error")
		mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			return nil, expectedErr
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})
		
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, result)
	})

	t.Run("passes through successful responses", func(t *testing.T) {
		expectedResult := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Success!"},
			},
		}
		mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			return expectedResult, nil
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})
		
		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("passes through error responses without auth failure", func(t *testing.T) {
		expectedResult := &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Some other error"},
			},
		}
		mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			return expectedResult, nil
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})
		
		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("intercepts exact GitHub auth error", func(t *testing.T) {
		mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Authentication Failed: Bad credentials"},
				},
			}, nil
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})
		
		assert.NoError(t, err)
		require.NotNil(t, result)
		
		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		require.Len(t, toolResult.Content, 1)
		
		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "GitHub authentication required")
	})

	t.Run("intercepts wrapped GitHub auth error", func(t *testing.T) {
		mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: `calling "tools/call": Authentication Failed: Bad credentials`},
				},
			}, nil
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})
		
		assert.NoError(t, err)
		require.NotNil(t, result)
		
		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		require.Len(t, toolResult.Content, 1)
		
		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "GitHub authentication required")
	})

	t.Run("intercepts auth error among multiple content items", func(t *testing.T) {
		mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "First message"},
					&mcp.TextContent{Text: "Authentication Failed: Bad credentials"},
					&mcp.TextContent{Text: "Third message"},
				},
			}, nil
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})
		
		assert.NoError(t, err)
		require.NotNil(t, result)
		
		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		require.Len(t, toolResult.Content, 1)
		
		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "GitHub authentication required")
	})

	t.Run("handles non-text content gracefully", func(t *testing.T) {
		mockHandler := func(ctx context.Context, session *mcp.ServerSession, method string, params mcp.Params) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.ImageContent{Data: []byte("base64data"), MIMEType: "image/png"},
					&mcp.TextContent{Text: "Authentication Failed: Bad credentials"},
				},
			}, nil
		}

		middleware := GitHubUnauthorizedMiddleware()
		wrappedHandler := middleware(mockHandler)

		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})
		
		assert.NoError(t, err)
		require.NotNil(t, result)
		
		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		assert.Contains(t, toolResult.Content[0].(*mcp.TextContent).Text, "GitHub authentication required")
	})
}