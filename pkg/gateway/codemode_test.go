package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/policy"
)

// TestCodemodeAdapter_PolicyEnforcement verifies that code-mode evaluates the
// ActionInvoke policy for the target backend tool before dispatching it, so a
// code-mode script cannot reach a tool that direct invocation / mcp-exec deny.
func TestCodemodeAdapter_PolicyEnforcement(t *testing.T) {
	newAdapter := func(mock *mockPolicyClient) *serverToolSetAdapter {
		g := &Gateway{
			policyClient: mock,
			configuration: Configuration{
				serverNames: []string{"backend-server"},
				servers:     map[string]catalog.Server{"backend-server": {Image: "img"}},
			},
		}
		sc, _, ok := g.configuration.Find("backend-server")
		require.True(t, ok)
		return &serverToolSetAdapter{gateway: g, serverName: "backend-server", serverConfig: sc}
	}

	t.Run("blocks_denied_tool", func(t *testing.T) {
		mock := newMockPolicyClient()
		mock.deny("backend-server", "dangerous-tool", policy.ActionInvoke, "tool blocked for safety")
		err := newAdapter(mock).checkInvokePolicy(context.Background(), "dangerous-tool", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy denied")
		assert.Contains(t, err.Error(), "dangerous-tool")
		assert.Contains(t, err.Error(), "tool blocked for safety")
	})

	t.Run("denies_on_error", func(t *testing.T) {
		mock := newMockPolicyClient()
		mock.failWith("backend-server", "dangerous-tool", policy.ActionInvoke, errors.New("policy service down"))
		err := newAdapter(mock).checkInvokePolicy(context.Background(), "dangerous-tool", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy")
	})

	t.Run("allows_permitted_tool", func(t *testing.T) {
		err := newAdapter(newMockPolicyClient()).checkInvokePolicy(context.Background(), "safe-tool", nil)
		require.NoError(t, err)
	})

	t.Run("nil_policy_client_allows", func(t *testing.T) {
		g := &Gateway{configuration: Configuration{
			serverNames: []string{"backend-server"},
			servers:     map[string]catalog.Server{"backend-server": {Image: "img"}},
		}}
		sc, _, ok := g.configuration.Find("backend-server")
		require.True(t, ok)
		a := &serverToolSetAdapter{gateway: g, serverName: "backend-server", serverConfig: sc}
		require.NoError(t, a.checkInvokePolicy(context.Background(), "any-tool", nil))
	})
}
