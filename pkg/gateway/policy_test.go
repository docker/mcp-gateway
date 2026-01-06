package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/policy"
)

// =============================================================================
// Mock Policy Client
// =============================================================================

// mockPolicyClient implements policy.Client for testing policy enforcement.
// It allows configuring specific decisions and errors for testing both
// deny scenarios and fail-open behavior.
type mockPolicyClient struct {
	decisions map[string]policy.Decision // "server:tool:action" -> decision
	errors    map[string]error           // "server:tool:action" -> error (for fail-open testing)
	fallback  policy.Decision            // default when no match
}

func newMockPolicyClient() *mockPolicyClient {
	return &mockPolicyClient{
		decisions: make(map[string]policy.Decision),
		errors:    make(map[string]error),
		fallback:  policy.Decision{Allowed: true},
	}
}

func (m *mockPolicyClient) Evaluate(_ context.Context, req policy.Request) (policy.Decision, error) {
	key := policyKey(req.Server, req.Tool, string(req.Action))
	if err, ok := m.errors[key]; ok {
		return policy.Decision{}, err
	}
	if dec, ok := m.decisions[key]; ok {
		return dec, nil
	}
	return m.fallback, nil
}

// deny configures the mock to deny a specific server/tool/action combination.
func (m *mockPolicyClient) deny(server, tool string, action policy.Action, reason string) {
	key := policyKey(server, tool, string(action))
	m.decisions[key] = policy.Decision{Allowed: false, Reason: reason}
}

// failWith configures the mock to return an error for a specific combination.
// This tests fail-open behavior: errors should allow the operation to proceed.
func (m *mockPolicyClient) failWith(server, tool string, action policy.Action, err error) {
	key := policyKey(server, tool, string(action))
	m.errors[key] = err
}

func policyKey(server, tool, action string) string {
	return fmt.Sprintf("%s:%s:%s", server, tool, action)
}

// =============================================================================
// L1/L2: FilterByPolicy Tests
// =============================================================================

func TestFilterByPolicy(t *testing.T) {
	t.Run("removes_denied_server", func(t *testing.T) {
		// Setup: two servers, one blocked
		mock := newMockPolicyClient()
		mock.deny("blocked-server", "", policy.ActionLoad, "admin blocked this server")

		cfg := &Configuration{
			serverNames: []string{"allowed-server", "blocked-server"},
			servers: map[string]catalog.Server{
				"allowed-server": {Image: "allowed-image"},
				"blocked-server": {Image: "blocked-image"},
			},
			config: make(map[string]map[string]any),
			tools: config.ToolsConfig{
				ServerTools: make(map[string][]string),
			},
		}

		// Execute
		err := cfg.FilterByPolicy(context.Background(), mock)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"allowed-server"}, cfg.serverNames, "blocked server should be removed")
		assert.Contains(t, cfg.servers, "allowed-server", "allowed server should remain")
		assert.NotContains(t, cfg.servers, "blocked-server", "blocked server should be removed from map")
	})

	t.Run("removes_denied_tool", func(t *testing.T) {
		// Setup: one server with two tools, one tool blocked
		mock := newMockPolicyClient()
		mock.deny("test-server", "blocked-tool", policy.ActionLoad, "tool not allowed")

		cfg := &Configuration{
			serverNames: []string{"test-server"},
			servers: map[string]catalog.Server{
				"test-server": {Image: "test-image"},
			},
			config: make(map[string]map[string]any),
			tools: config.ToolsConfig{
				ServerTools: map[string][]string{
					"test-server": {"allowed-tool", "blocked-tool"},
				},
			},
		}

		// Execute
		err := cfg.FilterByPolicy(context.Background(), mock)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"test-server"}, cfg.serverNames, "server should remain")
		require.Contains(t, cfg.tools.ServerTools, "test-server")
		assert.Equal(t, []string{"allowed-tool"}, cfg.tools.ServerTools["test-server"],
			"blocked tool should be removed, allowed tool should remain")
	})

	t.Run("nil_policy_client_allows_all", func(t *testing.T) {
		// Setup: nil policy client should be a no-op
		cfg := &Configuration{
			serverNames: []string{"server1", "server2"},
			servers: map[string]catalog.Server{
				"server1": {Image: "img1"},
				"server2": {Image: "img2"},
			},
			config: make(map[string]map[string]any),
			tools: config.ToolsConfig{
				ServerTools: make(map[string][]string),
			},
		}

		// Execute with nil
		err := cfg.FilterByPolicy(context.Background(), nil)

		// Assert: everything should remain
		require.NoError(t, err)
		assert.Len(t, cfg.serverNames, 2, "all servers should remain with nil policy client")
	})
}

// =============================================================================
// R1: McpServerToolHandler Tests
// =============================================================================

func TestMcpServerToolHandler_PolicyEnforcement(t *testing.T) {
	t.Run("blocks_denied_tool", func(t *testing.T) {
		// Setup
		mock := newMockPolicyClient()
		mock.deny("test-server", "test-tool", policy.ActionInvoke, "tool blocked by admin")

		g := &Gateway{
			policyClient: mock,
			configuration: Configuration{
				serverNames: []string{"test-server"},
				servers: map[string]catalog.Server{
					"test-server": {Image: "test-image"},
				},
			},
		}

		// Get handler
		handler := g.mcpServerToolHandler("test-server", nil, nil, "test-tool")

		// Execute
		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name: "test-tool",
			},
		}
		result, err := handler(context.Background(), req)

		// Assert: should return error with policy denial message
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "policy denied")
		assert.Contains(t, err.Error(), "test-tool")
		assert.Contains(t, err.Error(), "tool blocked by admin")
	})
	t.Run("denies_on_error", func(t *testing.T) {
		// Setup: policy returns error - should fail closed
		mock := newMockPolicyClient()
		mock.failWith("test-server", "test-tool", policy.ActionInvoke, errors.New("policy service down"))

		g := &Gateway{
			configuration: Configuration{
				serverNames: []string{"test-server"},
				servers: map[string]catalog.Server{
					"test-server": {Image: "img"},
				},
			},
			policyClient: mock,
		}

		handler := g.mcpServerToolHandler("test-server", nil, nil, "test-tool")

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name: "test-tool",
			},
		}

		_, err := handler(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy")
	})
}

// =============================================================================
// R3: McpExec Tests
// =============================================================================

func TestMcpExec_PolicyEnforcement(t *testing.T) {
	t.Run("blocks_denied_tool", func(t *testing.T) {
		// Setup
		mock := newMockPolicyClient()
		mock.deny("backend-server", "dangerous-tool", policy.ActionInvoke, "tool blocked for safety")

		mcpServer := mcp.NewServer(&mcp.Implementation{
			Name:    "test-gateway",
			Version: "1.0.0",
		}, nil)

		// Create a mock tool that should NOT be called
		toolCalled := false
		mockToolHandler := func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			toolCalled = true
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "executed"}},
			}, nil
		}

		g := &Gateway{
			policyClient:      mock,
			mcpServer:         mcpServer,
			toolRegistrations: make(map[string]ToolRegistration),
		}

		// Register the tool
		g.toolRegistrations["dangerous-tool"] = ToolRegistration{
			ServerName: "backend-server",
			Tool:       &mcp.Tool{Name: "dangerous-tool"},
			Handler:    mockToolHandler,
		}

		// Get mcp-exec handler
		mcpExecHandler := addMcpExecHandler(g)

		// Build request to execute dangerous-tool via mcp-exec
		execArgs := map[string]any{
			"name":      "dangerous-tool",
			"arguments": map[string]any{},
		}
		execArgsJSON, err := json.Marshal(execArgs)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "mcp-exec",
				Arguments: execArgsJSON,
			},
		}

		// Execute
		result, err := mcpExecHandler(context.Background(), req)

		// Assert
		require.NoError(t, err) // mcp-exec returns result, not error
		require.NotNil(t, result)
		assert.False(t, toolCalled, "tool should NOT have been called due to policy block")

		// Check result contains policy block message
		require.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "blocked by policy")
		assert.Contains(t, textContent.Text, "dangerous-tool")
	})

	t.Run("denies_on_error", func(t *testing.T) {
		// Setup: policy returns error - should fail closed and not execute tool
		mock := newMockPolicyClient()
		mock.failWith("backend-server", "test-tool", policy.ActionInvoke, errors.New("policy unavailable"))

		mcpServer := mcp.NewServer(&mcp.Implementation{
			Name:    "test-gateway",
			Version: "1.0.0",
		}, nil)

		toolCalled := false
		mockToolHandler := func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			toolCalled = true
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "tool executed successfully"}},
			}, nil
		}

		g := &Gateway{
			policyClient:      mock,
			mcpServer:         mcpServer,
			toolRegistrations: make(map[string]ToolRegistration),
			configuration: Configuration{
				serverNames: []string{"backend-server"},
				servers: map[string]catalog.Server{
					"backend-server": {Image: "img"},
				},
			},
		}

		g.toolRegistrations["test-tool"] = ToolRegistration{
			ServerName: "backend-server",
			Tool:       &mcp.Tool{Name: "test-tool"},
			Handler:    mockToolHandler,
		}

		mcpExecHandler := addMcpExecHandler(g)

		execArgs := map[string]any{
			"name":      "test-tool",
			"arguments": map[string]any{},
		}
		execArgsJSON, err := json.Marshal(execArgs)
		require.NoError(t, err)

		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      "mcp-exec",
				Arguments: execArgsJSON,
			},
		}

		result, err := mcpExecHandler(context.Background(), req)
		require.NoError(t, err) // handler returns JSON payload even on denial
		require.NotNil(t, result)
		assert.False(t, toolCalled, "tool should NOT be called when policy check errors (fail-closed)")

		require.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "policy")
	})
}

func TestMcpPrompts_PolicyEnforcement(t *testing.T) {
	// denied prompt should error
	mock := newMockPolicyClient()
	mock.deny("prompt-server", "summarize", policy.ActionPrompt, "prompt blocked")

	g := &Gateway{
		policyClient: mock,
		configuration: Configuration{
			serverNames: []string{"prompt-server"},
			servers: map[string]catalog.Server{
				"prompt-server": {Image: "img"},
			},
		},
	}

	h := g.mcpServerPromptHandler("prompt-server", nil)
	req := &mcp.GetPromptRequest{Params: &mcp.GetPromptParams{Name: "summarize"}}

	_, err := h(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy")
	assert.Contains(t, err.Error(), "summarize")
}
