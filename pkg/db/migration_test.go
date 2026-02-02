package db

import (
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to get test migrations filesystem
func getTestMigrationsFS() fs.FS {
	return os.DirFS("testdata/migrations")
}

// Helper to create a limited migration set (only migrations 001 and 002)
// Used for testing "database ahead" scenarios
func createLimitedMigrationsFS(t *testing.T) fs.FS {
	t.Helper()

	// Create temp directory
	tempDir := t.TempDir()

	// Copy only 001 and 002 from testdata/migrations
	migrations := []string{"001_create_users.up.sql", "002_create_posts.up.sql"}

	for _, file := range migrations {
		source := filepath.Join("testdata/migrations", file)
		dest := filepath.Join(tempDir, file)

		content, err := os.ReadFile(source)
		require.NoError(t, err)

		err = os.WriteFile(dest, content, 0o644)
		require.NoError(t, err)
	}

	return os.DirFS(tempDir)
}

func TestFreshDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	// Initialize fresh database
	dao, err := New(
		WithDatabaseFile(dbFile),
		WithMigrations(getTestMigrationsFS(), "."),
	)
	require.NoError(t, err)
	require.NotNil(t, dao)
	defer dao.Close()

	// Verify at latest version
	version := getDatabaseVersion(t, dbFile)
	assert.Equal(t, uint(3), version)

	// Verify migrations ran - tables should exist
	expectedTables := []string{"users", "posts"}
	for _, table := range expectedTables {
		exists := checkTableExists(t, dbFile, table)
		assert.True(t, exists, "table %s should exist", table)
	}
}

func TestDirtyDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	// Setup database in dirty state
	setupDatabaseAtVersion(t, dbFile, 1, true)

	// Attempt to initialize - should fail
	_, err := New(
		WithDatabaseFile(dbFile),
		WithMigrations(getTestMigrationsFS(), "."),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dirty state")
}

func TestConcurrentMigration(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	// Run multiple concurrent New() calls on a fresh database
	// With file locking, all should succeed
	// Without file locking, we'd expect "Dirty database version" errors
	const numConcurrent = 10
	var wg sync.WaitGroup
	errors := make(chan error, numConcurrent)
	daos := make([]DAO, numConcurrent)

	for i := range numConcurrent {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			dao, err := New(
				WithDatabaseFile(dbFile),
				WithMigrations(getTestMigrationsFS(), "."),
			)
			if err != nil {
				errors <- err
				return
			}
			daos[idx] = dao
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errList []error
	for err := range errors {
		errList = append(errList, err)
	}

	// All should succeed with file locking
	assert.Empty(t, errList, "all concurrent initializations should succeed")

	// Clean up
	for _, dao := range daos {
		if dao != nil {
			dao.Close()
		}
	}

	// Verify database is in good state
	version := getDatabaseVersion(t, dbFile)
	assert.Equal(t, uint(3), version)
}

func TestDatabaseAheadOfMigrationFiles(t *testing.T) {
	// Test scenario: Database is at version 3, but migration files only go up to version 2
	// This simulates running an older version of the code against a newer database
	// Expected: Error indicating the code version is too old for the database

	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	// Setup: Create database at version 3 (with all 3 tables)
	setupDatabaseAtVersion(t, dbFile, 3, false)

	// Verify initial state
	versionBefore := getDatabaseVersion(t, dbFile)
	assert.Equal(t, uint(3), versionBefore, "database should start at version 3")

	// Try to initialize with migrations that only go up to version 2
	// This should fail with a descriptive error
	_, err := New(
		WithDatabaseFile(dbFile),
		WithMigrations(createLimitedMigrationsFS(t), "."),
	)
	require.Error(t, err, "should error when database is ahead of migration files")

	// Verify error message is descriptive
	assert.Contains(t, err.Error(), "database version 3", "should mention current database version")
	assert.Contains(t, err.Error(), "ahead of the current application version", "should clearly state the problem")
	assert.Contains(t, err.Error(), dbFile, "should include database file path")
	assert.Contains(t, err.Error(), "upgrade to the latest version", "should suggest upgrade")

	// Verify database version stayed at 3 (no changes were made)
	versionAfter := getDatabaseVersion(t, dbFile)
	assert.Equal(t, uint(3), versionAfter, "database should remain at version 3")

	// Verify database is not dirty (error occurred before any migration attempts)
	dirty := getDatabaseDirtyState(t, dbFile)
	assert.False(t, dirty, "database should not be dirty")
}

// Helper functions

func setupDatabaseAtVersion(t *testing.T, dbFile string, version uint, dirty bool) {
	t.Helper()

	// Use golang-migrate to actually run migrations up to the specified version
	db, err := sql.Open("sqlite", "file:"+dbFile)
	require.NoError(t, err)
	defer db.Close()

	migDriver, err := iofs.New(getTestMigrationsFS(), ".")
	require.NoError(t, err)
	defer migDriver.Close()

	driver, err := msqlite.WithInstance(db, &msqlite.Config{})
	require.NoError(t, err)

	mig, err := migrate.NewWithInstance("iofs", migDriver, "sqlite", driver)
	require.NoError(t, err)

	// Migrate to the specific version
	err = mig.Migrate(version)
	require.NoError(t, err)

	if dirty {
		_, err = db.ExecContext(t.Context(), "UPDATE schema_migrations SET dirty = ? WHERE version = ?", true, version)
		require.NoError(t, err)
	}
}

func getDatabaseVersion(t *testing.T, dbFile string) uint {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+dbFile)
	require.NoError(t, err)
	defer db.Close()

	var version uint
	err = db.QueryRowContext(t.Context(), "SELECT version FROM schema_migrations").Scan(&version)
	require.NoError(t, err)

	return version
}

func getDatabaseDirtyState(t *testing.T, dbFile string) bool {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+dbFile)
	require.NoError(t, err)
	defer db.Close()

	var dirty bool
	err = db.QueryRowContext(t.Context(), "SELECT dirty FROM schema_migrations").Scan(&dirty)
	require.NoError(t, err)

	return dirty
}

func checkTableExists(t *testing.T, dbFile string, tableName string) bool {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+dbFile)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRowContext(
		t.Context(),
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
		tableName,
	).Scan(&count)
	require.NoError(t, err)

	return count > 0
}
