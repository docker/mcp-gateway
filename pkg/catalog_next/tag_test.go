package catalognext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/db"
)

func TestTag(t *testing.T) {
	ctx := t.Context()
	dao := setupTestDB(t)

	// First, create a catalog to tag
	sourceCatalog := &db.Catalog{
		Ref:    "mcp/test-catalog:v1",
		Source: SourcePrefixOCI + "mcp/original:latest",
		Title:  "Test Catalog v1",
	}
	err := dao.UpsertCatalog(ctx, *sourceCatalog)
	require.NoError(t, err)

	t.Run("tag catalog with new version", func(t *testing.T) {
		err := Tag(t.Context(), dao, "mcp/test-catalog:v1", "mcp/test-catalog:v2")
		require.NoError(t, err)

		// Verify the new catalog was created
		tagged, err := dao.GetCatalog(ctx, "mcp/test-catalog:v2")
		require.NoError(t, err)
		assert.Equal(t, "mcp/test-catalog:v2", tagged.Ref)
		assert.Equal(t, SourcePrefixOCI+"mcp/test-catalog:v1", tagged.Source)
	})

	t.Run("tag catalog with different name", func(t *testing.T) {
		err := Tag(ctx, dao, "mcp/test-catalog:v1", "mcp/prod-catalog:latest")
		require.NoError(t, err)

		// Verify the new catalog was created
		tagged, err := dao.GetCatalog(ctx, "mcp/prod-catalog:latest")
		require.NoError(t, err)
		assert.Equal(t, "mcp/prod-catalog:latest", tagged.Ref)
		assert.Equal(t, SourcePrefixOCI+"mcp/test-catalog:v1", tagged.Source)
	})

	t.Run("tag non-existent catalog fails", func(t *testing.T) {
		err := Tag(ctx, dao, "mcp/nonexistent:latest", "mcp/new:latest")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("tag with invalid source reference fails", func(t *testing.T) {
		err := Tag(ctx, dao, "invalid reference", "mcp/new:latest")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse oci-reference")
	})

	t.Run("tag with invalid target reference fails", func(t *testing.T) {
		err := Tag(ctx, dao, "mcp/test-catalog:v1", "invalid reference")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse tag")
	})
}
