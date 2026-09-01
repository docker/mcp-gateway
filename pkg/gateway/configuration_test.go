package gateway

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/docker"
)

func setupGatewayTestDB(t *testing.T) db.DAO {
	t.Helper()
	dao, err := db.New(db.WithDatabaseFile(filepath.Join(t.TempDir(), "test.db")))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, dao.Close())
	})
	return dao
}

func seedPulledCatalog(t *testing.T, dao db.DAO, ref, name, image string) {
	t.Helper()
	err := dao.UpsertCatalog(t.Context(), db.Catalog{
		Ref:    ref,
		Digest: "d-" + name,
		Title:  "pulled",
		Servers: []db.CatalogServer{{
			ServerType: "image",
			Image:      image,
			Snapshot: &db.ServerSnapshot{
				Server: catalog.Server{
					Name:  name,
					Type:  "server",
					Image: image,
				},
			},
		}},
	})
	require.NoError(t, err)
}

func TestReadOnceUsesPulledCatalogsForServers(t *testing.T) {
	dao := setupGatewayTestDB(t)
	seedPulledCatalog(t, dao, "docker.io/test/pulled:latest", "fetch", "mcp/fetch:pulled")
	seedPulledCatalog(t, dao, "docker.io/test/other:latest", "unused", "mcp/unused:latest")

	cfg := &FileBasedConfiguration{
		ServerNames: []string{"fetch"},
		catalogDAO:  dao,
	}
	got, err := cfg.readOnce(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []string{"fetch"}, got.ServerNames())
	server, _, found := got.Find("fetch")
	require.True(t, found)
	require.NotNil(t, server)
	assert.Equal(t, "mcp/fetch:pulled", server.Spec.Image)
	_, _, unusedFound := got.Find("unused")
	assert.True(t, unusedFound, "pulled catalogs should be available for lookup")
}

func TestReadOnceFileCatalogWinsOverPulledCatalog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	catalogsDir := filepath.Join(home, ".docker", "mcp", "catalogs")
	require.NoError(t, os.MkdirAll(catalogsDir, 0o755))
	err := os.WriteFile(filepath.Join(catalogsDir, "local.yaml"), []byte(`registry:
  fetch:
    type: server
    image: mcp/fetch:file
`), 0o644)
	require.NoError(t, err)

	dao := setupGatewayTestDB(t)
	seedPulledCatalog(t, dao, "docker.io/test/pulled:latest", "fetch", "mcp/fetch:pulled")

	cfg := &FileBasedConfiguration{
		CatalogPath: []string{"local.yaml"},
		ServerNames: []string{"fetch"},
		catalogDAO:  dao,
	}
	got, err := cfg.readOnce(t.Context())
	require.NoError(t, err)

	server, _, found := got.Find("fetch")
	require.True(t, found)
	require.NotNil(t, server)
	assert.Equal(t, "mcp/fetch:file", server.Spec.Image)
}

func TestMergePulledCatalogServersSkipsEmptySnapshots(t *testing.T) {
	dao := setupGatewayTestDB(t)
	err := dao.UpsertCatalog(t.Context(), db.Catalog{
		Ref:    "docker.io/test/empty:latest",
		Digest: "empty",
		Title:  "empty",
		Servers: []db.CatalogServer{{
			ServerType: "image",
			Image:      "mcp/orphan:latest",
		}},
	})
	require.NoError(t, err)

	servers := map[string]catalog.Server{}
	catalogs := map[string]string{}
	overrides := map[string]string{}
	cfg := &FileBasedConfiguration{catalogDAO: dao}
	require.NoError(t, cfg.mergePulledCatalogServers(t.Context(), servers, catalogs, overrides))
	assert.Empty(t, servers)
}

func TestReadServersFromOci(t *testing.T) {
	tests := []struct {
		name           string
		ociRef         string
		expectedCount  int
		expectError    bool
		skipIfNoDocker bool
	}{
		{
			name:           "valid OCI reference with one server",
			ociRef:         "index.docker.io/jimclark106/mcpservers@sha256:2455a592f3e919566ca8146a0e22058966550755f5a1106fb4dbc7f9465d43e9",
			expectedCount:  1,
			expectError:    false,
			skipIfNoDocker: true,
		},
		{
			name:          "empty OCI reference",
			ociRef:        "",
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:           "invalid OCI reference format",
			ociRef:         "invalid-reference",
			expectedCount:  0,
			expectError:    true,
			skipIfNoDocker: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipIfNoDocker {
				// Skip test if Docker is not available or if we can't access the registry
				// This is an integration test that requires network access
				if testing.Short() {
					t.Skip("Skipping integration test in short mode")
				}
			}

			// Create a FileBasedConfiguration with the OCI reference
			var ociRefs []string
			if tt.ociRef != "" {
				ociRefs = []string{tt.ociRef}
			}

			config := &FileBasedConfiguration{
				OciRef: ociRefs,
				docker: docker.NewClient(nil), // Use nil for default Docker client
			}

			ctx := context.Background()
			servers, _, err := config.readServersFromOci(ctx)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, servers, tt.expectedCount)

			// For the valid OCI reference test, verify we got a proper catalog.Server
			if tt.expectedCount > 0 {
				assert.NotEmpty(t, servers)

				// Check that we have at least one server in the map
				var foundServer catalog.Server
				var foundServerName string
				for name, server := range servers {
					foundServer = server
					foundServerName = name
					break
				}

				// Verify the server has some expected properties
				// The exact properties will depend on what's in the OCI artifact
				assert.NotEmpty(t, foundServerName, "Server should have a name")

				// We can't make specific assertions about the server content
				// without knowing exactly what's in the OCI artifact, but we can
				// verify it's a valid catalog.Server struct
				assert.IsType(t, catalog.Server{}, foundServer)
			}
		})
	}
}

func TestReadServersFromOciMultipleRefs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test with multiple OCI references (one valid, one empty)
	config := &FileBasedConfiguration{
		OciRef: []string{
			"", // empty reference should be skipped
			"index.docker.io/jimclark106/mcpservers@sha256:2455a592f3e919566ca8146a0e22058966550755f5a1106fb4dbc7f9465d43e9",
		},
		docker: docker.NewClient(nil),
	}

	ctx := context.Background()
	servers, _, err := config.readServersFromOci(ctx)

	require.NoError(t, err)
	assert.Len(t, servers, 1, "Should have exactly one server from the valid OCI reference")
}

func TestReadServersFromOciNoRefs(t *testing.T) {
	// Test with no OCI references
	config := &FileBasedConfiguration{
		OciRef: []string{},
		docker: docker.NewClient(nil),
	}

	ctx := context.Background()
	servers, _, err := config.readServersFromOci(ctx)

	require.NoError(t, err)
	assert.Empty(t, servers, "Should return empty map when no OCI references provided")
}
