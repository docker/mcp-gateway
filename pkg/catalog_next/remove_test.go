package catalognext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/workingset"
)

func TestRemove(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog
	catalog := Catalog{
		Name: "to-remove",
		Servers: []Server{
			{Type: workingset.ServerTypeImage, Image: "test:v1"},
		},
	}

	err := dao.CreateCatalog(ctx, catalog.ToDb())
	require.NoError(t, err)

	// Verify it exists
	retrieved, err := dao.GetCatalog(ctx, catalog.Digest())
	require.NoError(t, err)
	assert.Equal(t, catalog.Digest(), retrieved.Digest)

	// Remove it
	output := captureStdout(t, func() {
		err := Remove(ctx, dao, catalog.Digest())
		require.NoError(t, err)
	})

	// Verify output message
	assert.Contains(t, output, "Removed catalog "+catalog.Digest())

	// Verify it's gone
	_, err = dao.GetCatalog(ctx, catalog.Digest())
	require.Error(t, err)
}

func TestRemoveNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Remove(ctx, dao, "nonexistent-digest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog nonexistent-digest not found")
}

func TestRemoveMultipleCatalogs(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create multiple catalogs
	catalogs := []Catalog{
		{
			Name:    "catalog-1",
			Servers: []Server{{Type: workingset.ServerTypeImage, Image: "test:v1"}},
		},
		{
			Name:    "catalog-2",
			Servers: []Server{{Type: workingset.ServerTypeImage, Image: "test:v2"}},
		},
		{
			Name:    "catalog-3",
			Servers: []Server{{Type: workingset.ServerTypeImage, Image: "test:v3"}},
		},
	}

	for _, c := range catalogs {
		err := dao.CreateCatalog(ctx, c.ToDb())
		require.NoError(t, err)
	}

	// Verify all exist
	list, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 3)

	// Remove one
	output := captureStdout(t, func() {
		err := Remove(ctx, dao, catalogs[1].Digest())
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
	assert.True(t, digests[catalogs[0].Digest()])
	assert.False(t, digests[catalogs[1].Digest()])
	assert.True(t, digests[catalogs[2].Digest()])
}

func TestRemoveOutputFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Name: "test",
		Servers: []Server{
			{Type: workingset.ServerTypeImage, Image: "test:v1"},
		},
	}

	err := dao.CreateCatalog(ctx, catalog.ToDb())
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := Remove(ctx, dao, catalog.Digest())
		require.NoError(t, err)
	})

	// Verify output format
	expectedOutput := "Removed catalog " + catalog.Digest() + "\n"
	assert.Equal(t, expectedOutput, output)
}
