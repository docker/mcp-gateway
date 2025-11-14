package workingset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
)

var oneServerError = "at least one server must be specified"

func TestAddOneServerToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	servers := []string{
		"docker://myimage:latest",
	}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers, "", []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "My Image", dbSet.Servers[0].Snapshot.Server.Name)
}

func TestAddMultipleServersToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	servers := []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers, "", []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "My Image", dbSet.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "Another Image", dbSet.Servers[1].Snapshot.Server.Name)
}

func TestAddNoServersToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	servers := []string{}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers, "", []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), oneServerError)
}

func TestRemoveOneServerFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	serverURI := "docker://myimage:latest"
	setID := "test-set"

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", "test-set", []string{
		serverURI,
	})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, setID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)

	err = RemoveServers(ctx, dao, setID, []string{
		"My Image",
	})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, setID)
	require.NoError(t, err)

	assert.Empty(t, dbSet.Servers)
}

func TestRemoveMultipleServersFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSetID := "test-set"

	servers := []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetID, "My Test Set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 2)

	err = RemoveServers(ctx, dao, workingSetID, []string{"My Image", "Another Image"})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers)
}

func TestRemoveOneOfManyServerFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSetID := "test-set"

	servers := []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetID, "My Test Set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 2)

	err = RemoveServers(ctx, dao, workingSetID, []string{"My Image"})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "Another Image", dbSet.Servers[0].Snapshot.Server.Name)
}

func TestRemoveNoServersFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSetID := "test-set"

	servers := []string{
		"docker://myimage:latest",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetID, "My Test Set", servers)
	require.NoError(t, err)

	err = RemoveServers(ctx, dao, workingSetID, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), oneServerError)
}

func TestAddServersFromCatalog(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with test servers
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "catalog-server-1",
			serverType: "image",
			image:      "catalog-image-1:latest",
			tools:      []string{"tool1", "tool2"},
		},
		{
			name:       "catalog-server-2",
			serverType: "image",
			image:      "catalog-image-2:v1.0",
			tools:      []string{"tool3"},
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	// Add servers from catalog
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{}, catalog.Ref, []string{"catalog-server-1", "catalog-server-2"})
	require.NoError(t, err)

	// Verify servers were added
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Len(t, dbSet.Servers, 2)

	// Check first server
	assert.Equal(t, "image", dbSet.Servers[0].Type)
	assert.Equal(t, "catalog-image-1:latest", dbSet.Servers[0].Image)
	assert.Equal(t, "catalog-server-1", dbSet.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, []string{"tool1", "tool2"}, dbSet.Servers[0].Tools)
	assert.Equal(t, "default", dbSet.Servers[0].Secrets)

	// Check second server
	assert.Equal(t, "image", dbSet.Servers[1].Type)
	assert.Equal(t, "catalog-image-2:v1.0", dbSet.Servers[1].Image)
	assert.Equal(t, "catalog-server-2", dbSet.Servers[1].Snapshot.Server.Name)
	assert.Equal(t, []string{"tool3"}, dbSet.Servers[1].Tools)
	assert.Equal(t, "default", dbSet.Servers[1].Secrets)
}

func TestAddServersMixedDirectAndCatalog(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "catalog-server-1",
			serverType: "image",
			image:      "catalog-image-1:latest",
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	// Add both direct servers and catalog servers
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"docker://myimage:latest"}, catalog.Ref, []string{"catalog-server-1"})
	require.NoError(t, err)

	// Verify both types of servers were added
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Len(t, dbSet.Servers, 2)

	// First should be the direct server
	assert.Equal(t, "My Image", dbSet.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "myimage:latest", dbSet.Servers[0].Image)

	// Second should be from catalog
	assert.Equal(t, "catalog-server-1", dbSet.Servers[1].Snapshot.Server.Name)
	assert.Equal(t, "catalog-image-1:latest", dbSet.Servers[1].Image)
}

func TestAddServersFromCatalogMissingServer(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "catalog-server-1",
			serverType: "image",
			image:      "catalog-image-1:latest",
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to add a server that doesn't exist in the catalog
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{}, catalog.Ref, []string{"catalog-server-1", "nonexistent-server"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "servers were not found in catalog")
	assert.Contains(t, err.Error(), "nonexistent-server")
}

func TestAddServersFromCatalogInvalidDigest(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to add servers from a non-existent catalog
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{}, "invalid-digest", []string{"some-server"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog invalid-digest:latest not found")
}

func TestAddServersFromCatalogServersWithoutCatalog(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to add servers from a non-existent catalog
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{}, "", []string{"some-server"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog must be specified when adding catalog servers")
}

func TestAddServersFromCatalogWithoutDefaultSecret(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "catalog-server-1",
			serverType: "image",
			image:      "catalog-image-1:latest",
		},
	})

	// Create a working set without default secret
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{
			"custom": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	// Add server from catalog
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{}, catalog.Ref, []string{"catalog-server-1"})
	require.NoError(t, err)

	// Verify server was added without default secret
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Len(t, dbSet.Servers, 1)
	assert.Empty(t, dbSet.Servers[0].Secrets) // Should be empty string when no default
}

func TestAddServersFromCatalogEmptyCatalogServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog
	createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "catalog-server-1",
			serverType: "image",
			image:      "catalog-image-1:latest",
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to add with catalog ref but empty server list
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{}, "docker.io/test/catalog:latest", []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), oneServerError)
}

// Helper types and functions for catalog tests
type testCatalogServer struct {
	name       string
	serverType string
	image      string
	source     string
	tools      []string
}

func createTestCatalog(t *testing.T, dao db.DAO, servers []testCatalogServer) db.Catalog {
	t.Helper()

	catalogServers := make([]db.CatalogServer, len(servers))
	for i, server := range servers {
		catalogServers[i] = db.CatalogServer{
			ServerType: server.serverType,
			Tools:      server.tools,
			Source:     server.source,
			Image:      server.image,
			Snapshot: &db.ServerSnapshot{
				Server: catalog.Server{
					Name:  server.name,
					Type:  "server",
					Image: server.image,
				},
			},
		}
	}

	catalog := db.Catalog{
		Ref:     "test/catalog:latest",
		Digest:  "test-digest",
		Title:   "Test Catalog",
		Source:  "https://example.com/catalog",
		Servers: catalogServers,
	}

	err := dao.UpsertCatalog(t.Context(), catalog)
	require.NoError(t, err)

	return catalog
}
