package db

import (
	"database/sql"
	"testing"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCatalogAndGetCatalog(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Digest: "abc123",
		Name:   "test-catalog",
		Source: "https://example.com/catalog",
		Servers: []CatalogServer{
			{
				ServerType: "registry",
				Tools:      ToolList{"tool1", "tool2"},
				Source:     "https://example.com/server",
				Image:      "docker/test:latest",
			},
		},
	}

	err := dao.CreateCatalog(ctx, catalog)
	require.NoError(t, err)

	// Verify it was created
	retrieved, err := dao.GetCatalog(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, catalog.Digest, retrieved.Digest)
	assert.Equal(t, catalog.Name, retrieved.Name)
	assert.Equal(t, catalog.Source, retrieved.Source)
	assert.Len(t, retrieved.Servers, 1)
	assert.Equal(t, "registry", retrieved.Servers[0].ServerType)
}

func TestCreateCatalogWithEmptyServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Digest:  "empty123",
		Name:    "empty-catalog",
		Source:  "https://example.com/empty",
		Servers: []CatalogServer{},
	}

	err := dao.CreateCatalog(ctx, catalog)
	require.NoError(t, err)

	// Verify it was created
	retrieved, err := dao.GetCatalog(ctx, "empty123")
	require.NoError(t, err)
	assert.Equal(t, catalog.Digest, retrieved.Digest)
	assert.Equal(t, catalog.Name, retrieved.Name)
	// The Servers slice will be empty (not nil) when there are no servers
	assert.Empty(t, retrieved.Servers)
}

func TestCreateCatalogDuplicateDigest(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Digest:  "duplicate",
		Name:    "First",
		Source:  "https://example.com/first",
		Servers: []CatalogServer{},
	}

	err := dao.CreateCatalog(ctx, catalog)
	require.NoError(t, err)

	// Try to create another with the same digest
	catalog.Name = "Second"
	catalog.Source = "https://example.com/second"
	err = dao.CreateCatalog(ctx, catalog)
	require.Error(t, err)
	assert.True(t, IsDuplicateDigestError(err))
}

func TestGetCatalogNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	_, err := dao.GetCatalog(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDeleteCatalog(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Digest:  "delete123",
		Name:    "To Delete",
		Source:  "https://example.com/delete",
		Servers: []CatalogServer{},
	}

	err := dao.CreateCatalog(ctx, catalog)
	require.NoError(t, err)

	// Verify it exists
	_, err = dao.GetCatalog(ctx, "delete123")
	require.NoError(t, err)

	// Delete it
	err = dao.DeleteCatalog(ctx, "delete123")
	require.NoError(t, err)

	// Verify it's gone
	_, err = dao.GetCatalog(ctx, "delete123")
	require.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDeleteCatalogNonexistent(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Should not error even if it doesn't exist
	err := dao.DeleteCatalog(ctx, "nonexistent")
	require.NoError(t, err)
}

func TestListCatalogs(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create multiple catalogs
	catalogs := []Catalog{
		{
			Digest:  "list1",
			Name:    "First",
			Source:  "https://example.com/first",
			Servers: []CatalogServer{},
		},
		{
			Digest:  "list2",
			Name:    "Second",
			Source:  "https://example.com/second",
			Servers: []CatalogServer{},
		},
		{
			Digest:  "list3",
			Name:    "Third",
			Source:  "https://example.com/third",
			Servers: []CatalogServer{},
		},
	}

	for _, catalog := range catalogs {
		err := dao.CreateCatalog(ctx, catalog)
		require.NoError(t, err)
	}

	// List them
	retrieved, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, retrieved, 3)

	// Check that all digests are present
	digests := make(map[string]bool)
	for _, catalog := range retrieved {
		digests[catalog.Digest] = true
	}
	assert.True(t, digests["list1"])
	assert.True(t, digests["list2"])
	assert.True(t, digests["list3"])
}

func TestListCatalogsEmpty(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	retrieved, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Empty(t, retrieved)
}

func TestListCatalogsWithServersAndSnapshots(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog1 := Catalog{
		Digest: "withservers1",
		Name:   "Catalog 1",
		Source: "https://example.com/cat1",
		Servers: []CatalogServer{
			{
				ServerType: "registry",
				Tools:      ToolList{"tool1", "tool2"},
				Source:     "https://example.com/server1",
				Image:      "docker/test1:latest",
				Snapshot: &ServerSnapshot{
					Server: catalog.Server{
						Name:  "test-server",
						Type:  "server",
						Image: "docker/test1:latest",
					},
				},
			},
		},
	}

	catalog2 := Catalog{
		Digest: "withservers2",
		Name:   "Catalog 2",
		Source: "https://example.com/cat2",
		Servers: []CatalogServer{
			{
				ServerType: "image",
				Tools:      ToolList{"tool3"},
				Source:     "https://example.com/server2",
				Image:      "docker/test2:latest",
				Snapshot: &ServerSnapshot{
					Server: catalog.Server{
						Name:  "test-server",
						Type:  "server",
						Image: "docker/test2:latest",
					},
				},
			},
			{
				ServerType: "registry",
				Tools:      ToolList{"tool4", "tool5"},
				Source:     "https://example.com/server3",
				Image:      "docker/test3:latest",
				Snapshot: &ServerSnapshot{
					Server: catalog.Server{
						Name:  "test-server",
						Type:  "server",
						Image: "docker/test3:latest",
					},
				},
			},
		},
	}

	err := dao.CreateCatalog(ctx, catalog1)
	require.NoError(t, err)

	err = dao.CreateCatalog(ctx, catalog2)
	require.NoError(t, err)

	retrieved, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, retrieved, 2)

	// Find catalog 1 and verify
	var cat1 *Catalog
	for i := range retrieved {
		if retrieved[i].Digest == "withservers1" {
			cat1 = &retrieved[i]
			break
		}
	}
	require.NotNil(t, cat1)
	assert.Len(t, cat1.Servers, 1)
	assert.Equal(t, "registry", cat1.Servers[0].ServerType)
	assert.Equal(t, ToolList{"tool1", "tool2"}, cat1.Servers[0].Tools)
	assert.NotNil(t, cat1.Servers[0].Snapshot)
	assert.Equal(t, "test-server", cat1.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "server", cat1.Servers[0].Snapshot.Server.Type)
	assert.Equal(t, "docker/test1:latest", cat1.Servers[0].Snapshot.Server.Image)

	// Find catalog 2 and verify
	var cat2 *Catalog
	for i := range retrieved {
		if retrieved[i].Digest == "withservers2" {
			cat2 = &retrieved[i]
			break
		}
	}
	require.NotNil(t, cat2)
	assert.Len(t, cat2.Servers, 2)
}

func TestIsDuplicateDigestError(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Digest:  "errortest",
		Name:    "Error Test",
		Source:  "https://example.com/error",
		Servers: []CatalogServer{},
	}

	err := dao.CreateCatalog(ctx, catalog)
	require.NoError(t, err)

	// Try to create duplicate
	err = dao.CreateCatalog(ctx, catalog)
	require.Error(t, err)

	// Test the helper function
	assert.True(t, IsDuplicateDigestError(err))
}

func TestEmptyToolList(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Digest: "emptytools",
		Name:   "Empty Tools",
		Source: "https://example.com/emptytools",
		Servers: []CatalogServer{
			{
				ServerType: "registry",
				Tools:      ToolList{},
				Source:     "https://example.com/server",
				Image:      "docker/test:latest",
			},
		},
	}

	err := dao.CreateCatalog(ctx, catalog)
	require.NoError(t, err)

	retrieved, err := dao.GetCatalog(ctx, "emptytools")
	require.NoError(t, err)
	assert.Len(t, retrieved.Servers, 1)
	assert.NotNil(t, retrieved.Servers[0].Tools)
	assert.Empty(t, retrieved.Servers[0].Tools)
}
