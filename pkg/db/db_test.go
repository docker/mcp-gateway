package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCreatesDirectoryWhenNotExists(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a nested path that doesn't exist yet
	nonExistentDir := filepath.Join(tempDir, "nested", "directories", "that", "dont", "exist")
	dbFile := filepath.Join(nonExistentDir, "test.db")

	// Verify the directory doesn't exist before creating the database
	_, err := os.Stat(nonExistentDir)
	assert.True(t, os.IsNotExist(err), "Directory should not exist before database creation")

	// Create the database - this should create the directory
	dao, err := New(WithDatabaseFile(dbFile))
	require.NoError(t, err)
	require.NotNil(t, dao)

	// Verify the directory was created
	stat, err := os.Stat(nonExistentDir)
	require.NoError(t, err, "Directory should exist after database creation")
	assert.True(t, stat.IsDir(), "Created path should be a directory")
}
