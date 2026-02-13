package catalognext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/docker/mcp-gateway/test/mocks"
)

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
	dao := setupTestDB(t)
	ctx := t.Context()
	ociService := mocks.NewMockOCIService()

	// writeCommunityToDatabase simulates what PullCommunity does to the DB
	err := writeCommunityToDatabase(ctx, dao, "registry.modelcontextprotocol.io", map[string]catalog.Server{
		"test": {Name: "test", Type: "server", Image: "test:latest"},
	})
	require.NoError(t, err)

	// Verify IsAPIRegistry correctly identifies the hostname
	require.True(t, IsAPIRegistry("registry.modelcontextprotocol.io"))
	require.True(t, IsAPIRegistry("registry.modelcontextprotocol.io:latest"))

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
