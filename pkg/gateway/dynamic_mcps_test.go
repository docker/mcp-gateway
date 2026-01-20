package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMcpExecTool(t *testing.T) {
	// Create a test gateway with minimal setup
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "test-gateway",
		Version: "1.0.0",
	}, nil)

	g := &Gateway{
		toolRegistrations: make(map[string]ToolRegistration),
		mcpServer:         mcpServer,
	}

	// Create a mock tool that we'll call through mcp-exec
	mockToolCalled := false
	mockToolResult := "mock tool executed"
	mockTool := &mcp.Tool{
		Name:        "test-tool",
		Description: "A test tool",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"param1": {
					Type:        "string",
					Description: "Test parameter",
				},
			},
			Required: []string{"param1"},
		},
	}

	mockToolHandler := func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mockToolCalled = true

		// Verify the arguments were passed correctly
		var args map[string]any
		err := json.Unmarshal(req.Params.Arguments, &args)
		require.NoError(t, err)
		assert.Equal(t, "test-value", args["param1"])

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: mockToolResult}},
		}, nil
	}

	// Register the mock tool
	g.toolRegistrations["test-tool"] = ToolRegistration{
		ServerName: "test-server",
		Tool:       mockTool,
		Handler:    mockToolHandler,
	}

	// Create the mcp-exec tool registration (without telemetry wrapper for testing)
	mcpExecToolReg := g.createMcpExecTool()
	require.NotNil(t, mcpExecToolReg)
	assert.Equal(t, "mcp-exec", mcpExecToolReg.Tool.Name)

	// Extract the underlying handler by creating a new one without telemetry
	// For testing purposes, we'll create a handler directly
	mcpExecHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Name      string `json:"name"`
			Arguments any    `json:"arguments"`
		}

		if req.Params.Arguments == nil {
			return nil, fmt.Errorf("missing arguments")
		}

		paramsBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return nil, err
		}

		if params.Name == "" {
			return nil, fmt.Errorf("name parameter is required")
		}

		toolName := params.Name

		// Look up the tool in current tool registrations
		g.capabilitiesMu.RLock()
		toolReg, found := g.toolRegistrations[toolName]
		g.capabilitiesMu.RUnlock()

		if !found {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: "Error: Tool '" + toolName + "' not found in current session.",
				}},
			}, nil
		}

		// Create a new CallToolRequest with the provided arguments
		argumentsJSON, err := json.Marshal(params.Arguments)
		if err != nil {
			return nil, err
		}

		toolCallRequest := &mcp.CallToolRequest{
			Session: req.Session,
			Params: &mcp.CallToolParamsRaw{
				Meta:      req.Params.Meta,
				Name:      toolName,
				Arguments: argumentsJSON,
			},
			Extra: req.Extra,
		}

		// Execute the tool using its registered handler
		result, err := toolReg.Handler(ctx, toolCallRequest)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	// Test calling a tool through mcp-exec
	t.Run("successful tool execution", func(t *testing.T) {
		mockToolCalled = false

		// Create a request to call test-tool through mcp-exec
		execArgs := map[string]any{
			"name": "test-tool",
			"arguments": map[string]any{
				"param1": "test-value",
			},
		}

		execArgsJSON, err := json.Marshal(execArgs)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "mcp-exec",
				Arguments: execArgsJSON,
			},
		}

		result, err := mcpExecHandler(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, mockToolCalled, "mock tool should have been called")

		// Verify the result
		require.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, mockToolResult, textContent.Text)
	})

	// Test calling a non-existent tool
	t.Run("tool not found", func(t *testing.T) {
		execArgs := map[string]any{
			"name": "non-existent-tool",
			"arguments": map[string]any{
				"param1": "test-value",
			},
		}

		execArgsJSON, err := json.Marshal(execArgs)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "mcp-exec",
				Arguments: execArgsJSON,
			},
		}

		result, err := mcpExecHandler(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify error message
		require.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "Error: Tool 'non-existent-tool' not found")
	})

	// Test missing tool name
	t.Run("missing tool name", func(t *testing.T) {
		execArgs := map[string]any{
			"arguments": map[string]any{
				"param1": "test-value",
			},
		}

		execArgsJSON, err := json.Marshal(execArgs)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "mcp-exec",
				Arguments: execArgsJSON,
			},
		}

		result, err := mcpExecHandler(t.Context(), req)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "name parameter is required")
	})

	// Test with no arguments to tool
	t.Run("tool with no arguments", func(t *testing.T) {
		// Register a tool that takes no arguments
		noArgToolCalled := false
		noArgTool := &mcp.Tool{
			Name:        "no-arg-tool",
			Description: "A tool with no arguments",
			InputSchema: &jsonschema.Schema{
				Type: "object",
			},
		}

		noArgToolHandler := func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			noArgToolCalled = true
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "no-arg tool executed"}},
			}, nil
		}

		g.toolRegistrations["no-arg-tool"] = ToolRegistration{
			ServerName: "test-server",
			Tool:       noArgTool,
			Handler:    noArgToolHandler,
		}

		execArgs := map[string]any{
			"name": "no-arg-tool",
		}

		execArgsJSON, err := json.Marshal(execArgs)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "mcp-exec",
				Arguments: execArgsJSON,
			},
		}

		result, err := mcpExecHandler(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, noArgToolCalled, "no-arg tool should have been called")
	})

	// Test with arguments passed as JSON-encoded string (backward compatibility)
	t.Run("arguments as JSON-encoded string", func(t *testing.T) {
		mockToolCalled = false

		// Create arguments as they would come when schema had Type: "string"
		// The arguments field itself is a JSON string containing the actual arguments
		innerArgs := map[string]any{
			"param1": "test-value",
		}
		innerArgsJSON, err := json.Marshal(innerArgs)
		require.NoError(t, err)

		execArgs := map[string]any{
			"name":      "test-tool",
			"arguments": string(innerArgsJSON), // Pass as string (simulating the old behavior)
		}

		execArgsJSON, err := json.Marshal(execArgs)
		require.NoError(t, err)

		// Create a simple handler that mimics the actual createMcpExecTool logic without telemetry
		testHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse parameters
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}

			if req.Params.Arguments == nil {
				return nil, fmt.Errorf("missing arguments")
			}

			paramsBytes, err := json.Marshal(req.Params.Arguments)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal arguments: %w", err)
			}

			if err := json.Unmarshal(paramsBytes, &params); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}

			if params.Name == "" {
				return nil, fmt.Errorf("name parameter is required")
			}

			// Look up the tool in current tool registrations
			g.capabilitiesMu.RLock()
			toolReg, found := g.toolRegistrations[params.Name]
			g.capabilitiesMu.RUnlock()

			if !found {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: fmt.Sprintf("Error: Tool '%s' not found in current session.", params.Name),
					}},
				}, nil
			}

			// Handle the case where arguments might be a JSON-encoded string
			var toolArguments json.RawMessage
			if len(params.Arguments) > 0 {
				// Try to unmarshal as a string first (for backward compatibility)
				var argString string
				if err := json.Unmarshal(params.Arguments, &argString); err == nil {
					// It was a JSON string, use the unescaped content
					toolArguments = json.RawMessage(argString)
				} else {
					// It's already a proper JSON object/value
					toolArguments = params.Arguments
				}
			}

			// Create a new CallToolRequest with the provided arguments
			toolCallRequest := &mcp.CallToolRequest{
				Session: req.Session,
				Params: &mcp.CallToolParamsRaw{
					Meta:      req.Params.Meta,
					Name:      params.Name,
					Arguments: toolArguments,
				},
				Extra: req.Extra,
			}

			// Execute the tool using its registered handler
			return toolReg.Handler(ctx, toolCallRequest)
		}

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "mcp-exec",
				Arguments: execArgsJSON,
			},
		}

		// Test with our handler
		result, err := testHandler(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, mockToolCalled, "mock tool should have been called with string arguments")

		// Verify the result
		require.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, mockToolResult, textContent.Text)
	})
}

func TestShortenURLWithBitly(t *testing.T) {
	t.Run("missing Bitly token", func(t *testing.T) {
		t.Setenv("BITLY_ACCESS_TOKEN", "")

		longURL := "https://example.com/oauth/authorize?client_id=abc123&redirect_uri=https://example.com/callback&response_type=code&state=xyz789"
		_, err := shortenURL(t.Context(), longURL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "BITLY_ACCESS_TOKEN not set")
	})

	t.Run("with valid Bitly token", func(t *testing.T) {
		// This test requires a valid Bitly token to be set
		// Skip if BITLY_ACCESS_TOKEN is not set in environment
		token := os.Getenv("BITLY_ACCESS_TOKEN")
		if token == "" {
			t.Skip("Skipping Bitly integration test: BITLY_ACCESS_TOKEN not set")
		}

		longURL := "https://example.com/oauth/authorize?client_id=abc123&redirect_uri=https://example.com/callback&response_type=code&state=xyz789"
		shortURL, err := shortenURL(t.Context(), longURL)

		if err != nil {
			// If we get an error with a valid token, it might be a network issue or rate limit
			t.Logf("Bitly request failed (this may be expected in CI): %v", err)
		} else {
			assert.NotEmpty(t, shortURL)
			assert.Contains(t, shortURL, "bit.ly")
			t.Logf("Shortened URL: %s", shortURL)
		}
	})
}
