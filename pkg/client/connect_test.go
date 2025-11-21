package client

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/db"
)

func setupTestDB(t *testing.T) (db.DAO, func()) {
	t.Helper()

	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	dao, err := db.New(db.WithDatabaseFile(dbFile))
	require.NoError(t, err)

	// Override newDAO to use the test database
	originalNewDAO := newDAO
	newDAO = func(_ ...db.Option) (db.DAO, error) {
		return db.New(db.WithDatabaseFile(dbFile))
	}

	cleanup := func() {
		dao.Close()
		newDAO = originalNewDAO
	}

	return dao, cleanup
}

func TestConnectWithNonExistingProfile(t *testing.T) {
	_, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	err := Connect(ctx, "/tmp", Config{}, "cursor", false, "nonexistent-profile")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "profile 'nonexistent-profile' not found")
}

func TestConnectWithExistingProfile(t *testing.T) {
	dao, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	workingSet := db.WorkingSet{
		ID:      "test-profile",
		Name:    "Test Profile",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	err = Connect(ctx, "/tmp", Config{}, "cursor", false, "test-profile")
	if err != nil {
		assert.NotContains(t, err.Error(), "profile 'test-profile' not found")
	}
}
