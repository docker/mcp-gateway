package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

func TestCreateCatalogAndGetCatalog(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Ref:    "docker.io/test/catalog:latest",
		Digest: "abc123",
		Title:  "test-catalog",
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

	err := dao.UpsertCatalog(ctx, catalog)
	require.NoError(t, err)

	// Verify it was created
	retrieved, err := dao.GetCatalog(ctx, "docker.io/test/catalog:latest")
	require.NoError(t, err)
	assert.Equal(t, catalog.Digest, retrieved.Digest)
	assert.Equal(t, catalog.Title, retrieved.Title)
	assert.Equal(t, catalog.Source, retrieved.Source)
	assert.Len(t, retrieved.Servers, 1)
	assert.Equal(t, "registry", retrieved.Servers[0].ServerType)
}

func TestCreateCatalogWithEmptyServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Ref:     "docker.io/test/empty:latest",
		Digest:  "empty123",
		Title:   "empty-catalog",
		Source:  "https://example.com/empty",
		Servers: []CatalogServer{},
	}

	err := dao.UpsertCatalog(ctx, catalog)
	require.NoError(t, err)

	// Verify it was created
	retrieved, err := dao.GetCatalog(ctx, "docker.io/test/empty:latest")
	require.NoError(t, err)
	assert.Equal(t, catalog.Digest, retrieved.Digest)
	assert.Equal(t, catalog.Title, retrieved.Title)
	// The Servers slice will be empty (not nil) when there are no servers
	assert.Empty(t, retrieved.Servers)
}

func TestUpsertCatalogReplaceExisting(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Ref:     "docker.io/test/duplicate:latest",
		Digest:  "duplicate",
		Title:   "First",
		Source:  "https://example.com/first",
		Servers: []CatalogServer{},
	}

	err := dao.UpsertCatalog(ctx, catalog)
	require.NoError(t, err)

	// Upsert another with the same ref but different digest
	catalog.Digest = "duplicate2"
	catalog.Title = "Second"
	catalog.Source = "https://example.com/second"
	err = dao.UpsertCatalog(ctx, catalog)
	require.NoError(t, err)

	// Verify it was updated
	retrieved, err := dao.GetCatalog(ctx, "docker.io/test/duplicate:latest")
	require.NoError(t, err)
	assert.Equal(t, "duplicate2", retrieved.Digest)
	assert.Equal(t, "Second", retrieved.Title)
	assert.Equal(t, "https://example.com/second", retrieved.Source)
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
		Ref:     "docker.io/test/delete:latest",
		Digest:  "delete123",
		Title:   "To Delete",
		Source:  "https://example.com/delete",
		Servers: []CatalogServer{},
	}

	err := dao.UpsertCatalog(ctx, catalog)
	require.NoError(t, err)

	// Verify it exists
	_, err = dao.GetCatalog(ctx, "docker.io/test/delete:latest")
	require.NoError(t, err)

	// Delete it
	err = dao.DeleteCatalog(ctx, "docker.io/test/delete:latest")
	require.NoError(t, err)

	// Verify it's gone
	_, err = dao.GetCatalog(ctx, "docker.io/test/delete:latest")
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
			Ref:     "docker.io/test/first:latest",
			Digest:  "list1",
			Title:   "First",
			Source:  "https://example.com/first",
			Servers: []CatalogServer{},
		},
		{
			Ref:     "docker.io/test/second:latest",
			Digest:  "list2",
			Title:   "Second",
			Source:  "https://example.com/second",
			Servers: []CatalogServer{},
		},
		{
			Ref:     "docker.io/test/third:latest",
			Digest:  "list3",
			Title:   "Third",
			Source:  "https://example.com/third",
			Servers: []CatalogServer{},
		},
	}

	for _, catalog := range catalogs {
		err := dao.UpsertCatalog(ctx, catalog)
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
		Ref:    "docker.io/test/catalog1:latest",
		Digest: "withservers1",
		Title:  "Catalog 1",
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
		Ref:    "docker.io/test/catalog2:latest",
		Digest: "withservers2",
		Title:  "Catalog 2",
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

	err := dao.UpsertCatalog(ctx, catalog1)
	require.NoError(t, err)

	err = dao.UpsertCatalog(ctx, catalog2)
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

func TestEmptyToolList(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Ref:    "docker.io/test/emptytools:latest",
		Digest: "emptytools",
		Title:  "Empty Tools",
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

	err := dao.UpsertCatalog(ctx, catalog)
	require.NoError(t, err)

	retrieved, err := dao.GetCatalog(ctx, "docker.io/test/emptytools:latest")
	require.NoError(t, err)
	assert.Len(t, retrieved.Servers, 1)
	assert.NotNil(t, retrieved.Servers[0].Tools)
	assert.Empty(t, retrieved.Servers[0].Tools)
}
