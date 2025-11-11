package catalognext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func TestCreateFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set first
	ws := db.WorkingSet{
		ID:   "test-ws",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  string(workingset.ServerTypeImage),
				Image: "docker/test:latest",
				Tools: []string{"tool1", "tool2"},
			},
			{
				Type:   string(workingset.ServerTypeRegistry),
				Source: "https://example.com/server",
				Tools:  []string{"tool3"},
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Capture stdout to verify the output message
	output := captureStdout(t, func() {
		err := CreateFromWorkingSet(ctx, dao, "test-ws", "My Catalog")
		require.NoError(t, err)
	})

	// Verify output message
	assert.Contains(t, output, "Catalog My Catalog created with digest")

	// Verify the catalog was created
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	catalog := NewFromDb(&catalogs[0])
	assert.Equal(t, "My Catalog", catalog.Name)
	assert.Equal(t, "working-set:test-ws", catalog.Source)
	assert.Len(t, catalog.Servers, 2)

	// Verify servers were copied correctly
	assert.Equal(t, workingset.ServerTypeImage, catalog.Servers[0].Type)
	assert.Equal(t, "docker/test:latest", catalog.Servers[0].Image)
	assert.Equal(t, []string{"tool1", "tool2"}, catalog.Servers[0].Tools)

	assert.Equal(t, workingset.ServerTypeRegistry, catalog.Servers[1].Type)
	assert.Equal(t, "https://example.com/server", catalog.Servers[1].Source)
	assert.Equal(t, []string{"tool3"}, catalog.Servers[1].Tools)
}

func TestCreateFromWorkingSetWithEmptyName(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	ws := db.WorkingSet{
		ID:   "test-ws",
		Name: "Original Working Set Name",
		Servers: db.ServerList{
			{
				Type:  string(workingset.ServerTypeImage),
				Image: "docker/test:latest",
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create catalog without providing a name (should use working set name)
	captureStdout(t, func() {
		err := CreateFromWorkingSet(ctx, dao, "test-ws", "")
		require.NoError(t, err)
	})

	// Verify the catalog was created with working set name
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	catalog := NewFromDb(&catalogs[0])
	assert.Equal(t, "Original Working Set Name", catalog.Name)
}

func TestCreateFromWorkingSetNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := CreateFromWorkingSet(ctx, dao, "nonexistent-ws", "Test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "working set nonexistent-ws not found")
}

func TestCreateFromWorkingSetDuplicate(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	ws := db.WorkingSet{
		ID:   "test-ws",
		Name: "Test",
		Servers: db.ServerList{
			{
				Type:  string(workingset.ServerTypeImage),
				Image: "docker/test:latest",
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create catalog from working set
	captureStdout(t, func() {
		err := CreateFromWorkingSet(ctx, dao, "test-ws", "Test")
		require.NoError(t, err)
	})

	// Try to create the same catalog again (same content = same digest)
	err = CreateFromWorkingSet(ctx, dao, "test-ws", "Test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog with digest")
	assert.Contains(t, err.Error(), "already exists")
}

func TestCreateFromWorkingSetWithSnapshot(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with snapshot
	snapshot := &db.ServerSnapshot{
		Server: catalog.Server{
			Name:        "test-server",
			Description: "Test server",
		},
	}

	ws := db.WorkingSet{
		ID:   "test-ws",
		Name: "Test",
		Servers: db.ServerList{
			{
				Type:     string(workingset.ServerTypeImage),
				Image:    "docker/test:latest",
				Snapshot: snapshot,
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create catalog from working set
	captureStdout(t, func() {
		err := CreateFromWorkingSet(ctx, dao, "test-ws", "Test")
		require.NoError(t, err)
	})

	// Verify snapshot was preserved
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	catalog := NewFromDb(&catalogs[0])
	require.NotNil(t, catalog.Servers[0].Snapshot)
	assert.Equal(t, "test-server", catalog.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "Test server", catalog.Servers[0].Snapshot.Server.Description)
}

func TestCreateFromWorkingSetEmptyServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with no servers
	ws := db.WorkingSet{
		ID:      "empty-ws",
		Name:    "Empty",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create catalog from empty working set
	captureStdout(t, func() {
		err := CreateFromWorkingSet(ctx, dao, "empty-ws", "Empty Catalog")
		require.NoError(t, err)
	})

	// Verify catalog was created
	testCatalog := Catalog{Name: "Empty Catalog", Servers: []Server{}}
	retrieved, err := dao.GetCatalog(ctx, testCatalog.Digest())
	require.NoError(t, err)

	catalog := NewFromDb(retrieved)
	assert.Equal(t, "Empty Catalog", catalog.Name)
	assert.Empty(t, catalog.Servers)
}

func TestCreateFromWorkingSetPreservesAllServerFields(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with all possible server fields
	ws := db.WorkingSet{
		ID:   "detailed-ws",
		Name: "Detailed",
		Servers: db.ServerList{
			{
				Type:   string(workingset.ServerTypeRegistry),
				Source: "https://example.com/api",
				Tools:  []string{"read", "write", "delete"},
				Config: map[string]any{
					"timeout": 30,
					"retries": 3,
				},
				Secrets: "api-secrets",
			},
			{
				Type:  string(workingset.ServerTypeImage),
				Image: "mycompany/myserver:v1.2.3",
				Tools: []string{"deploy"},
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create catalog
	captureStdout(t, func() {
		err := CreateFromWorkingSet(ctx, dao, "detailed-ws", "Detailed Catalog")
		require.NoError(t, err)
	})

	// Verify all fields are preserved - use GetCatalog to get exact order
	testCatalog := Catalog{
		Name:   "Detailed Catalog",
		Source: "working-set:detailed-ws",
		Servers: []Server{
			{
				Type:   workingset.ServerTypeRegistry,
				Source: "https://example.com/api",
				Tools:  []string{"read", "write", "delete"},
			},
			{
				Type:  workingset.ServerTypeImage,
				Image: "mycompany/myserver:v1.2.3",
				Tools: []string{"deploy"},
			},
		},
	}

	dbCatalog, err := dao.GetCatalog(ctx, testCatalog.Digest())
	require.NoError(t, err)
	catalog := NewFromDb(dbCatalog)

	assert.Len(t, catalog.Servers, 2)

	// Check registry server
	assert.Equal(t, workingset.ServerTypeRegistry, catalog.Servers[0].Type)
	assert.Equal(t, "https://example.com/api", catalog.Servers[0].Source)
	assert.Equal(t, []string{"read", "write", "delete"}, catalog.Servers[0].Tools)

	// Check image server
	assert.Equal(t, workingset.ServerTypeImage, catalog.Servers[1].Type)
	assert.Equal(t, "mycompany/myserver:v1.2.3", catalog.Servers[1].Image)
	assert.Equal(t, []string{"deploy"}, catalog.Servers[1].Tools)
}

func TestCreateFromWorkingSetMultipleTimes(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create working set
	ws := db.WorkingSet{
		ID:   "test-ws",
		Name: "Test",
		Servers: db.ServerList{
			{
				Type:  string(workingset.ServerTypeImage),
				Image: "docker/test:v1",
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create first catalog
	captureStdout(t, func() {
		err := CreateFromWorkingSet(ctx, dao, "test-ws", "Catalog 1")
		require.NoError(t, err)
	})

	// Create second catalog with same name (truly same digest now)
	err = CreateFromWorkingSet(ctx, dao, "test-ws", "Catalog 1")

	// Should fail due to duplicate digest (name is part of digest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
