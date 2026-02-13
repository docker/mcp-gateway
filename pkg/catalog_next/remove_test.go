package catalognext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func TestRemove(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog
	catalog := Catalog{
		Ref: "test/to-remove:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "to-remove",
			Servers: []Server{
				{Type: workingset.ServerTypeImage, Image: "test:v1"},
			},
		},
	}

	dbCat, err := catalog.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	// Verify it exists
	retrieved, err := dao.GetCatalog(ctx, catalog.Ref)
	require.NoError(t, err)
	digest, err := catalog.Digest()
	require.NoError(t, err)
	assert.Equal(t, digest, retrieved.Digest)

	// Remove it
	output := captureStdout(t, func() {
		err := Remove(ctx, dao, catalog.Ref)
		require.NoError(t, err)
	})

	// Verify output message
	assert.Contains(t, output, "Removed catalog "+catalog.Ref)

	// Verify it's gone
	_, err = dao.GetCatalog(ctx, catalog.Ref)
	require.Error(t, err)
}

func TestRemoveCommunityRegistry(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Seed a community registry catalog
	err := writeCommunityToDatabase(ctx, dao, "registry.modelcontextprotocol.io", map[string]catalog.Server{
		"test-server": {Name: "test-server", Type: "server", Image: "test:latest"},
	})
	require.NoError(t, err)

	// Verify it exists
	_, err = dao.GetCatalog(ctx, "registry.modelcontextprotocol.io")
	require.NoError(t, err)

	// Remove using the community hostname
	output := captureStdout(t, func() {
		err := Remove(ctx, dao, "registry.modelcontextprotocol.io")
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Removed catalog registry.modelcontextprotocol.io")

	// Verify it's gone
	_, err = dao.GetCatalog(ctx, "registry.modelcontextprotocol.io")
	require.Error(t, err)
}

func TestRemoveNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Remove(ctx, dao, "test/nonexistent:latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog test/nonexistent:latest not found")
}

func TestRemoveMultipleCatalogs(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create multiple catalogs (use normalized refs without docker.io prefix)
	catalogs := []Catalog{
		{
			Ref: "test/catalog-1:latest",
			CatalogArtifact: CatalogArtifact{
				Title:   "catalog-1",
				Servers: []Server{{Type: workingset.ServerTypeImage, Image: "test:v1"}},
			},
		},
		{
			Ref: "test/catalog-2:latest",
			CatalogArtifact: CatalogArtifact{
				Title:   "catalog-2",
				Servers: []Server{{Type: workingset.ServerTypeImage, Image: "test:v2"}},
			},
		},
		{
			Ref: "test/catalog-3:latest",
			CatalogArtifact: CatalogArtifact{
				Title:   "catalog-3",
				Servers: []Server{{Type: workingset.ServerTypeImage, Image: "test:v3"}},
			},
		},
	}

	for _, c := range catalogs {
		dbCat, err := c.ToDb()
		require.NoError(t, err)
		err = dao.UpsertCatalog(ctx, dbCat)
		require.NoError(t, err)
	}

	// Verify all exist
	list, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 3)

	// Remove one
	output := captureStdout(t, func() {
		err := Remove(ctx, dao, catalogs[1].Ref)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Removed catalog")

	// Verify only two remain
	list, err = dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Verify the correct one was removed
	digests := make(map[string]bool)
	for _, c := range list {
		digests[c.Digest] = true
	}
	digest0, err := catalogs[0].Digest()
	require.NoError(t, err)
	digest1, err := catalogs[1].Digest()
	require.NoError(t, err)
	digest2, err := catalogs[2].Digest()
	require.NoError(t, err)
	assert.True(t, digests[digest0])
	assert.False(t, digests[digest1])
	assert.True(t, digests[digest2])
}

func TestRemoveOutputFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Ref: "test/test:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "test",
			Servers: []Server{
				{Type: workingset.ServerTypeImage, Image: "test:v1"},
			},
		},
	}

	dbCat, err := catalog.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := Remove(ctx, dao, catalog.Ref)
		require.NoError(t, err)
	})

	// Verify output format
	expectedOutput := "Removed catalog " + catalog.Ref + "\n"
	assert.Equal(t, expectedOutput, output)
}
