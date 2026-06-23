package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/policy"
)

func newServerLoadPolicyGateway(mock *mockPolicyClient) *Gateway {
	return &Gateway{
		policyClient: mock,
		configuration: Configuration{
			serverNames: []string{"backend-server"},
			servers:     map[string]catalog.Server{"backend-server": {Image: "img"}},
			config:      map[string]map[string]any{},
		},
	}
}

// TestCheckServerLoadPolicy covers the shared ActionLoad gate used by the
// dynamic management tools (mcp-config-set / activate-profile).
func TestCheckServerLoadPolicy(t *testing.T) {
	t.Run("blocks_denied_server", func(t *testing.T) {
		mock := newMockPolicyClient()
		mock.deny("backend-server", "", policy.ActionLoad, "server blocked by admin")
		err := newServerLoadPolicyGateway(mock).checkServerLoadPolicy(context.Background(), "backend-server", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocked by policy")
	})
	t.Run("denies_on_error", func(t *testing.T) {
		mock := newMockPolicyClient()
		mock.failWith("backend-server", "", policy.ActionLoad, errors.New("policy service down"))
		err := newServerLoadPolicyGateway(mock).checkServerLoadPolicy(context.Background(), "backend-server", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy")
	})
	t.Run("allows_permitted_server", func(t *testing.T) {
		require.NoError(t, newServerLoadPolicyGateway(newMockPolicyClient()).checkServerLoadPolicy(context.Background(), "backend-server", nil))
	})
	t.Run("nil_policy_client_allows", func(t *testing.T) {
		g := &Gateway{configuration: Configuration{
			serverNames: []string{"backend-server"},
			servers:     map[string]catalog.Server{"backend-server": {Image: "img"}},
		}}
		require.NoError(t, g.checkServerLoadPolicy(context.Background(), "backend-server", nil))
	})
}

// TestConfigSet_PolicyEnforcement verifies mcp-config-set refuses to mutate a
// server's config when policy denies it (and applies it when allowed).
func TestConfigSet_PolicyEnforcement(t *testing.T) {
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "mcp-config-set",
		Arguments: json.RawMessage(`{"server":"backend-server","config":{"k":"v"}}`),
	}}

	t.Run("denied_is_blocked_and_not_applied", func(t *testing.T) {
		mock := newMockPolicyClient()
		mock.deny("backend-server", "", policy.ActionLoad, "server blocked by admin")
		g := newServerLoadPolicyGateway(mock)
		res, err := configSetHandler(g)(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.True(t, res.IsError)
		assert.Empty(t, g.configuration.config, "config must not be written when policy denies")
	})
	t.Run("allowed_is_applied", func(t *testing.T) {
		g := newServerLoadPolicyGateway(newMockPolicyClient())
		res, err := configSetHandler(g)(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.False(t, res.IsError)
		assert.NotEmpty(t, g.configuration.config, "config should be written when allowed")
	})
}
