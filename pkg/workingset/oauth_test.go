package workingset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/oauth"
)

// mockDesktopMode overrides the package-level function vars so the test
// runs as if Docker Desktop is present. Call the returned cleanup func
// (or use t.Cleanup) to restore the originals.
func mockDesktopMode(t *testing.T) {
	t.Helper()
	oldCE := isCEModeFunc
	oldSnapshot := registerWithSnapshotFunc
	oldDiscovery := registerForDynamicDiscoveryFunc

	isCEModeFunc = func() bool { return false }

	t.Cleanup(func() {
		isCEModeFunc = oldCE
		registerWithSnapshotFunc = oldSnapshot
		registerForDynamicDiscoveryFunc = oldDiscovery
	})
}

func TestRegisterOAuthProvidersForServers_CEModeSkipsAll(t *testing.T) {
	oldCE := isCEModeFunc
	isCEModeFunc = func() bool { return true }
	t.Cleanup(func() { isCEModeFunc = oldCE })

	// Track registration calls — none should happen in CE mode.
	var snapshotCalls, discoveryCalls int
	registerWithSnapshotFunc = func(_ context.Context, _, _ string) error {
		snapshotCalls++
		return nil
	}
	registerForDynamicDiscoveryFunc = func(_ context.Context, _, _ string) error {
		discoveryCalls++
		return nil
	}
	t.Cleanup(func() {
		registerWithSnapshotFunc = oauth.RegisterProviderWithSnapshot
		registerForDynamicDiscoveryFunc = oauth.RegisterProviderForDynamicDiscovery
	})

	servers := []Server{
		{
			Type: ServerTypeRemote,
			Snapshot: &ServerSnapshot{
				Server: catalog.Server{
					Name: "oauth-server",
					Type: "remote",
					OAuth: &catalog.OAuth{
						Providers: []catalog.OAuthProvider{{Provider: "github"}},
					},
				},
			},
		},
		{
			Type: ServerTypeRemote,
			Snapshot: &ServerSnapshot{
				Server: catalog.Server{
					Name:   "dynamic-server",
					Type:   "remote",
					Remote: catalog.Remote{URL: "https://example.com/mcp"},
				},
			},
		},
	}

	RegisterOAuthProvidersForServers(context.Background(), servers)

	assert.Equal(t, 0, snapshotCalls, "no snapshot registration in CE mode")
	assert.Equal(t, 0, discoveryCalls, "no dynamic discovery in CE mode")
}

func TestRegisterOAuthProvidersForServers_NilSnapshotSkipped(t *testing.T) {
	mockDesktopMode(t)

	var snapshotCalls int
	registerWithSnapshotFunc = func(_ context.Context, _, _ string) error {
		snapshotCalls++
		return nil
	}
	registerForDynamicDiscoveryFunc = func(_ context.Context, _, _ string) error {
		return nil
	}

	servers := []Server{
		{Type: ServerTypeRemote, Snapshot: nil},
	}

	RegisterOAuthProvidersForServers(context.Background(), servers)

	assert.Equal(t, 0, snapshotCalls, "nil snapshot should be skipped")
}

func TestRegisterOAuthProvidersForServers_ExplicitOAuthRegistered(t *testing.T) {
	mockDesktopMode(t)

	var registeredServers []string
	registerWithSnapshotFunc = func(_ context.Context, serverName, _ string) error {
		registeredServers = append(registeredServers, serverName)
		return nil
	}
	registerForDynamicDiscoveryFunc = func(_ context.Context, _, _ string) error {
		return nil
	}

	servers := []Server{
		{
			Type: ServerTypeRemote,
			Snapshot: &ServerSnapshot{
				Server: catalog.Server{
					Name: "catalog-oauth",
					Type: "remote",
					OAuth: &catalog.OAuth{
						Providers: []catalog.OAuthProvider{{Provider: "github"}},
					},
				},
			},
		},
	}

	RegisterOAuthProvidersForServers(context.Background(), servers)

	assert.Equal(t, []string{"catalog-oauth"}, registeredServers)
}

func TestRegisterOAuthProvidersForServers_DynamicDiscovery(t *testing.T) {
	mockDesktopMode(t)

	var discoveredServers []string
	registerWithSnapshotFunc = func(_ context.Context, _, _ string) error {
		return nil
	}
	registerForDynamicDiscoveryFunc = func(_ context.Context, serverName, _ string) error {
		discoveredServers = append(discoveredServers, serverName)
		return nil
	}

	servers := []Server{
		{
			Type: ServerTypeRemote,
			Snapshot: &ServerSnapshot{
				Server: catalog.Server{
					Name:   "remote-server",
					Type:   "remote",
					Remote: catalog.Remote{URL: "https://example.com/mcp"},
				},
			},
		},
	}

	RegisterOAuthProvidersForServers(context.Background(), servers)

	assert.Equal(t, []string{"remote-server"}, discoveredServers)
}

func TestRegisterOAuthProvidersForServers_NonRemoteSkipped(t *testing.T) {
	mockDesktopMode(t)

	var snapshotCalls, discoveryCalls int
	registerWithSnapshotFunc = func(_ context.Context, _, _ string) error {
		snapshotCalls++
		return nil
	}
	registerForDynamicDiscoveryFunc = func(_ context.Context, _, _ string) error {
		discoveryCalls++
		return nil
	}

	servers := []Server{
		{
			Type: ServerTypeImage,
			Snapshot: &ServerSnapshot{
				Server: catalog.Server{
					Name:  "container-server",
					Type:  "server",
					Image: "docker/server:latest",
				},
			},
		},
	}

	RegisterOAuthProvidersForServers(context.Background(), servers)

	assert.Equal(t, 0, snapshotCalls, "non-remote server should not trigger snapshot registration")
	assert.Equal(t, 0, discoveryCalls, "non-remote server should not trigger dynamic discovery")
}
