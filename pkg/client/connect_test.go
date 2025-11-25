package client

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/db"
)

func setupTestDB(t *testing.T) db.DAO {
	t.Helper()

	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	dao, err := db.New(db.WithDatabaseFile(dbFile))
	require.NoError(t, err)

	t.Cleanup(func() {
		dao.Close()
	})

	return dao
}

func TestConnectWithNonExistingProfile(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Connect(ctx, dao, "/tmp", Config{}, "cursor", false, "nonexistent-profile")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get profile: nonexistent-profile")
}

func TestConnectWithExistingProfile(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSet := db.WorkingSet{
		ID:      "test-profile",
		Name:    "Test Profile",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	err = Connect(ctx, dao, "/tmp", Config{}, "cursor", true, "test-profile")
	assert.NotContains(t, err.Error(), "failed to get profile: test")
}
