package workingset

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveExistingWorkingSet(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Verify it exists
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	// Remove it
	err = Remove(ctx, dao, "test-set")
	require.NoError(t, err)

	// Verify it's gone
	_, err = dao.GetWorkingSet(ctx, "test-set")
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.True(t, errors.Is(err, sql.ErrNoRows))
}

func TestRemoveNonExistentWorkingSet(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Try to remove a non-existent working set
	err := Remove(ctx, dao, "non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRemoveOneOfMany(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create multiple working sets
	for i := 1; i <= 3; i++ {
		err := dao.CreateWorkingSet(ctx, db.WorkingSet{
			ID:      string(rune('a'+i-1)) + "-set",
			Name:    "Set " + string(rune('0'+i)),
			Servers: db.ServerList{},
			Secrets: db.SecretMap{},
		})
		require.NoError(t, err)
	}

	// Remove the middle one
	err := Remove(ctx, dao, "b-set")
	require.NoError(t, err)

	// Verify only b-set is gone
	aSet, err := dao.GetWorkingSet(ctx, "a-set")
	require.NoError(t, err)
	assert.NotNil(t, aSet)

	_, err = dao.GetWorkingSet(ctx, "b-set")
	require.ErrorIs(t, err, sql.ErrNoRows)

	cSet, err := dao.GetWorkingSet(ctx, "c-set")
	require.NoError(t, err)
	assert.NotNil(t, cSet)

	// Verify count
	sets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Len(t, sets, 2)
}

func TestRemoveTwice(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Remove it
	err = Remove(ctx, dao, "test-set")
	require.NoError(t, err)

	// Try to remove it again
	err = Remove(ctx, dao, "test-set")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRemoveCaseSensitive(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to remove with wrong case
	err = Remove(ctx, dao, "TEST-SET")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify original still exists
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.NotNil(t, dbSet)
}

func TestRemoveWithEmptyId(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Try to remove with empty ID
	err := Remove(ctx, dao, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
