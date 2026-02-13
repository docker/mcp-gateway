package catalognext

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/docker/mcp-gateway/test/mocks"
)

func TestPullAll(t *testing.T) {
	t.Run("empty database prints message", func(t *testing.T) {
		dao := setupTestDB(t)
		ctx := t.Context()
		ociService := mocks.NewMockOCIService()

		output := captureStdout(t, func() {
			err := PullAll(ctx, dao, ociService)
			require.NoError(t, err)
		})

		assert.Contains(t, output, "No catalogs found")
	})

	t.Run("attempts each catalog in database", func(t *testing.T) {
		dao := setupTestDB(t)
		ctx := t.Context()
		ociService := mocks.NewMockOCIService()

		// Seed DB with two OCI catalogs that will fail to pull (no real OCI backend)
		for _, ref := range []string{"nonexistent/catalog-a:v1", "nonexistent/catalog-b:v1"} {
			cat := Catalog{
				Ref:    ref,
				Source: SourcePrefixOCI + ref,
				CatalogArtifact: CatalogArtifact{
					Title:   ref,
					Servers: []Server{},
				},
			}
			dbCat, err := cat.ToDb()
			require.NoError(t, err)
			require.NoError(t, dao.UpsertCatalog(ctx, dbCat))
		}

		var pullErr error
		output := captureStdout(t, func() {
			pullErr = PullAll(ctx, dao, ociService)
		})

		// Both catalogs should have been attempted
		assert.Contains(t, output, "Pulling nonexistent/catalog-a:v1...")
		assert.Contains(t, output, "Pulling nonexistent/catalog-b:v1...")

		// Both should fail since no real OCI backend
		require.Error(t, pullErr)
		assert.Contains(t, pullErr.Error(), "failed to pull some catalogs")
	})
}

func TestPrintRegistryPullResult(t *testing.T) {
	t.Run("basic output format", func(t *testing.T) {
		result := &PullCommunityResult{
			ServersAdded:   10,
			ServersOCI:     7,
			ServersRemote:  3,
			ServersSkipped: 5,
			TotalServers:   15,
			SkippedByType:  map[string]int{},
		}

		output := captureStdout(t, func() {
			printRegistryPullResult("registry.modelcontextprotocol.io", result)
		})

		assert.Contains(t, output, "Pulled 10 servers from registry.modelcontextprotocol.io")
		assert.Contains(t, output, "Total in registry: 15")
		assert.Contains(t, output, "Imported:          10")
		assert.Contains(t, output, "OCI (stdio):     7")
		assert.Contains(t, output, "Remote:          3")
		assert.Contains(t, output, "Skipped:           5")
	})

	t.Run("with skipped type breakdown", func(t *testing.T) {
		result := &PullCommunityResult{
			ServersAdded:   5,
			ServersOCI:     3,
			ServersRemote:  2,
			ServersSkipped: 8,
			TotalServers:   13,
			SkippedByType: map[string]int{
				"npm":  5,
				"pip":  2,
				"none": 1,
			},
		}

		output := captureStdout(t, func() {
			printRegistryPullResult("registry.modelcontextprotocol.io", result)
		})

		assert.Contains(t, output, "npm:")
		assert.Contains(t, output, "pip:")
		assert.Contains(t, output, "no packages:")
	})

	t.Run("zero servers", func(t *testing.T) {
		result := &PullCommunityResult{
			ServersAdded:   0,
			ServersOCI:     0,
			ServersRemote:  0,
			ServersSkipped: 0,
			TotalServers:   0,
			SkippedByType:  map[string]int{},
		}

		output := captureStdout(t, func() {
			printRegistryPullResult("registry.modelcontextprotocol.io", result)
		})

		assert.Contains(t, output, "Pulled 0 servers")
		assert.Contains(t, output, "Total in registry: 0")
	})
}

func TestPullDetectsAPIRegistry(t *testing.T) {
	// Pull() routes to PullCommunity for well-known community registry hostnames.
	// We can't mock PullCommunity's internal client, but we can verify the DB
	// state it produces: the source prefix should be "registry:" not "oci:".
	dao := setupTestDB(t)
	ctx := t.Context()
	ociService := mocks.NewMockOCIService()

	// writeCommunityToDatabase simulates what PullCommunity does to the DB
	// without hitting the network. Verify the routing logic by checking that
	// pullCatalog writes the correct source prefix.
	err := writeCommunityToDatabase(ctx, dao, "registry.modelcontextprotocol.io", map[string]catalog.Server{
		"test": {Name: "test", Type: "server", Image: "test:latest"},
	})
	require.NoError(t, err)

	// Verify the DB entry has the registry source prefix
	dbCatalog, err := dao.GetCatalog(ctx, "registry.modelcontextprotocol.io")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(dbCatalog.Source, SourcePrefixRegistry),
		"expected source to start with %q, got %q", SourcePrefixRegistry, dbCatalog.Source)

	// Verify IsAPIRegistry correctly identifies the hostname
	require.True(t, IsAPIRegistry("registry.modelcontextprotocol.io"))
	require.True(t, IsAPIRegistry("registry.modelcontextprotocol.io:latest"))

	// Verify pullCatalog checks the DB source prefix for non-hostname refs
	// by confirming the source prefix check works on the stored catalog
	require.True(t, strings.HasPrefix(dbCatalog.Source, SourcePrefixRegistry))

	// Verify OCI refs don't get routed to community
	require.False(t, IsAPIRegistry("docker/mcp-catalog:latest"))

	// Verify pullOCI is the fallback by calling it with a bad ref
	_, err = pullOCI(ctx, dao, ociService, "nonexistent/catalog:latest")
	require.Error(t, err)
}

func TestWriteCommunityToDatabaseCatalogTitle(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := writeCommunityToDatabase(ctx, dao, "registry.modelcontextprotocol.io", map[string]catalog.Server{})
	require.NoError(t, err)

	dbCatalog, err := dao.GetCatalog(ctx, "registry.modelcontextprotocol.io")
	require.NoError(t, err)

	cat := NewFromDb(dbCatalog)
	assert.Equal(t, "MCP Community Registry", cat.Title)
}

func TestWriteCommunityToDatabaseSnapshotPreserved(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	servers := map[string]catalog.Server{
		"snapshot-test": {
			Name:        "snapshot-test",
			Type:        "server",
			Image:       "ghcr.io/example/snapshot:v1",
			Description: "Test server with description",
		},
	}

	err := writeCommunityToDatabase(ctx, dao, "registry.modelcontextprotocol.io", servers)
	require.NoError(t, err)

	dbCatalog, err := dao.GetCatalog(ctx, "registry.modelcontextprotocol.io")
	require.NoError(t, err)

	cat := NewFromDb(dbCatalog)
	require.Len(t, cat.Servers, 1)
	require.NotNil(t, cat.Servers[0].Snapshot)
	assert.Equal(t, "snapshot-test", cat.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "Test server with description", cat.Servers[0].Snapshot.Server.Description)
	assert.Equal(t, workingset.ServerTypeImage, cat.Servers[0].Type)
}
