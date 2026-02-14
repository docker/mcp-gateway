package oauth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oauthhelpers "github.com/docker/mcp-gateway-oauth-helpers"
	"github.com/docker/mcp-gateway/pkg/desktop"
)

// mockDCRClient implements dcrRegistrationClient for testing.
type mockDCRClient struct {
	clients    map[string]*desktop.DCRClient
	registered map[string]desktop.RegisterDCRRequest
}

func newMockDCRClient() *mockDCRClient {
	return &mockDCRClient{
		clients:    make(map[string]*desktop.DCRClient),
		registered: make(map[string]desktop.RegisterDCRRequest),
	}
}

func (m *mockDCRClient) GetDCRClient(_ context.Context, app string) (*desktop.DCRClient, error) {
	c, ok := m.clients[app]
	if !ok {
		return nil, errors.New("not found")
	}
	return c, nil
}

func (m *mockDCRClient) RegisterDCRClientPending(_ context.Context, app string, req desktop.RegisterDCRRequest) error {
	m.registered[app] = req
	return nil
}

// mockProber implements oauthProber for testing.
type mockProber struct {
	discovery *oauthhelpers.Discovery
	err       error
}

func (m *mockProber) DiscoverOAuthRequirements(_ context.Context, _ string) (*oauthhelpers.Discovery, error) {
	return m.discovery, m.err
}

func TestRegisterProviderForDynamicDiscovery_SkipsAlreadyRegistered(t *testing.T) {
	client := newMockDCRClient()
	client.clients["my-server"] = &desktop.DCRClient{State: "unregistered"}

	prober := &mockProber{} // should not be called

	err := registerProviderForDynamicDiscovery(t.Context(), "my-server", "https://example.com/mcp", client, prober)
	require.NoError(t, err)
	assert.Empty(t, client.registered, "should not register when already exists")
}

func TestRegisterProviderForDynamicDiscovery_RegistersWhenOAuthRequired(t *testing.T) {
	client := newMockDCRClient()
	prober := &mockProber{
		discovery: &oauthhelpers.Discovery{RequiresOAuth: true},
	}

	err := registerProviderForDynamicDiscovery(t.Context(), "ai-kubit-mcp-server", "https://mcp.kubit.ai/mcp", client, prober)
	require.NoError(t, err)

	req, ok := client.registered["ai-kubit-mcp-server"]
	require.True(t, ok, "should have registered DCR client")
	assert.Equal(t, "ai-kubit-mcp-server", req.ProviderName, "provider name should match server name")
}

func TestRegisterProviderForDynamicDiscovery_SkipsWhenNoOAuthRequired(t *testing.T) {
	client := newMockDCRClient()
	prober := &mockProber{
		discovery: &oauthhelpers.Discovery{RequiresOAuth: false},
	}

	err := registerProviderForDynamicDiscovery(t.Context(), "my-server", "https://example.com/mcp", client, prober)
	require.NoError(t, err)
	assert.Empty(t, client.registered, "should not register when OAuth not required")
}

func TestRegisterProviderForDynamicDiscovery_SkipsOnProbeError(t *testing.T) {
	client := newMockDCRClient()
	prober := &mockProber{
		err: errors.New("connection refused"),
	}

	err := registerProviderForDynamicDiscovery(t.Context(), "my-server", "https://unreachable.example.com/mcp", client, prober)
	require.NoError(t, err)
	assert.Empty(t, client.registered, "should not register when probe fails")
}
