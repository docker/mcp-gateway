package interceptors

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallbacksWithOAuthInterceptorEnabled(t *testing.T) {
	// When oauth-interceptor is enabled
	middlewares := Callbacks(false, false, true, nil)

	// Should have the GitHub interceptor
	assert.Len(t, middlewares, 1, "should have GitHub interceptor when enabled")

	// Actually test the middleware behavior with a 401 error
	mockHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "failed to get user: GET https://api.github.com/user: 401 Bad credentials []"},
			},
		}, nil
	}

	// Apply the middleware
	wrappedHandler := middlewares[0](mockHandler)
	result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

	// Should intercept and return OAuth URL
	require.NoError(t, err)
	toolResult, ok := result.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, toolResult.Content, 1)
	textContent, ok := toolResult.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "GitHub authentication required")
}

func TestCallbacksWithOAuthInterceptorDisabled(t *testing.T) {
	// When oauth-interceptor is disabled
	middlewares := Callbacks(false, false, false, nil)

	// Should have no middlewares
	assert.Empty(t, middlewares, "should have no GitHub interceptor when disabled")
}

func TestCallbacksEndToEndWithFeatureToggle(t *testing.T) {
	github401Error := &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: "failed to get user: GET https://api.github.com/user: 401 Bad credentials []"},
		},
	}

	// Create mock handler that returns GitHub 401 error (success call with error result)
	createMockHandler := func() func(context.Context, *mcp.ServerSession, string, mcp.Params) (mcp.Result, error) {
		return func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return github401Error, nil
		}
	}

	t.Run("with feature enabled - should intercept", func(t *testing.T) {
		mockHandler := createMockHandler()

		middlewares := Callbacks(false, false, true, nil) // OAuth enabled
		require.NotEmpty(t, middlewares)

		wrappedHandler := middlewares[0](mockHandler)
		result, err := wrappedHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		require.NoError(t, err)
		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)

		// Should have intercepted and changed the response
		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "GitHub authentication required")
		assert.NotEqual(t, github401Error, result, "response should be modified")
	})

	t.Run("with feature disabled - should pass through", func(t *testing.T) {
		mockHandler := createMockHandler()

		middlewares := Callbacks(false, false, false, nil) // OAuth disabled

		// No middleware means the handler runs unchanged
		if len(middlewares) == 0 {
			// Simulate what would happen - error passes through
			result, err := mockHandler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})
			require.NoError(t, err)
			assert.Equal(t, github401Error, result, "401 error should pass through unchanged")
		}
	})
}

func TestOAuthInterceptorIntegration(t *testing.T) {
	// Test that when enabled, OAuth interceptor is first in the chain
	// and actually intercepts 401 errors

	t.Run("enabled - intercepts GitHub 401", func(t *testing.T) {
		// Create a handler that returns GitHub 401
		baseHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "GET https://api.github.com/user: 401 Bad credentials"},
				},
			}, nil
		}

		// Get middlewares with OAuth enabled
		middlewares := Callbacks(true, true, true, nil) // logCalls, blockSecrets, oauthEnabled

		// Apply all middlewares
		handler := baseHandler
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}

		// Call the wrapped handler
		result, err := handler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		// Should have intercepted
		require.NoError(t, err)
		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "GitHub authentication required")
	})

	t.Run("disabled - passes through GitHub 401", func(t *testing.T) {
		originalError := &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "GET https://api.github.com/user: 401 Bad credentials"},
			},
		}

		baseHandler := func(_ context.Context, _ *mcp.ServerSession, _ string, _ mcp.Params) (mcp.Result, error) {
			return originalError, nil
		}

		// Get middlewares with OAuth disabled
		middlewares := Callbacks(true, true, false, nil) // logCalls, blockSecrets, oauthDisabled

		// Apply all middlewares (OAuth interceptor won't be in the chain)
		handler := baseHandler
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}

		// Call the wrapped handler
		result, err := handler(context.Background(), nil, "tools/call", &mcp.CallToolParams{})

		// Error should pass through unchanged (except for logging)
		require.NoError(t, err)
		toolResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		assert.True(t, toolResult.IsError)
		// The content should still contain the original error
		textContent, ok := toolResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "401 Bad credentials")
		assert.NotContains(t, textContent.Text, "GitHub authentication required")
	})
}

func TestCallbacksOAuthInterceptorWithOtherMiddleware(t *testing.T) {
	// Test that OAuth interceptor plays nicely with other middleware

	// With OAuth enabled and logCalls enabled
	middlewares := Callbacks(true, false, true, nil)
	assert.Len(t, middlewares, 2, "should have both GitHub interceptor and log calls middleware")

	// With OAuth disabled but logCalls enabled
	middlewares = Callbacks(true, false, false, nil)
	assert.Len(t, middlewares, 1, "should only have log calls middleware")
}
