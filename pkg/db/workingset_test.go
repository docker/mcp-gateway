package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) DAO {
	t.Helper()

	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	dao, err := New(WithDatabaseFile(dbFile))
	require.NoError(t, err)

	return dao
}

func TestCreateWorkingSetAndGetWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSet := WorkingSet{
		ID:   "test-id",
		Name: "Test Working Set",
		Servers: ServerList{
			{
				Type:   "registry",
				Source: "https://example.com/server",
				Config: map[string]any{"key": "value"},
				Tools:  []string{"tool1", "tool2"},
			},
		},
		Secrets: SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	// Verify it was created
	retrieved, err := dao.GetWorkingSet(ctx, "test-id")
	require.NoError(t, err)
	assert.Equal(t, workingSet.ID, retrieved.ID)
	assert.Equal(t, workingSet.Name, retrieved.Name)
}

func TestCreateWorkingSetWithEmptyServersAndSecrets(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSet := WorkingSet{
		ID:      "empty-id",
		Name:    "Empty Working Set",
		Servers: ServerList{},
		Secrets: SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	// Verify it was created
	retrieved, err := dao.GetWorkingSet(ctx, "empty-id")
	require.NoError(t, err)
	assert.Equal(t, workingSet.ID, retrieved.ID)
	assert.Equal(t, workingSet.Name, retrieved.Name)
	assert.NotNil(t, retrieved.Servers)
	assert.NotNil(t, retrieved.Secrets)
	assert.Len(t, retrieved.Servers, 0)
	assert.Len(t, retrieved.Secrets, 0)
}

func TestCreateWorkingSetDuplicateID(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSet := WorkingSet{
		ID:      "duplicate-id",
		Name:    "First",
		Servers: ServerList{},
		Secrets: SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	// Try to create another with the same ID
	workingSet.Name = "Second"
	err = dao.CreateWorkingSet(ctx, workingSet)
	assert.Error(t, err)
}

func TestGetWorkingSetNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	_, err := dao.GetWorkingSet(ctx, "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUpdateWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create initial working set
	workingSet := WorkingSet{
		ID:   "update-test",
		Name: "Original Name",
		Servers: ServerList{
			{Type: "registry", Source: "https://example.com/original"},
		},
		Secrets: SecretMap{
			"secret1": {Provider: "docker-desktop-store"},
		},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	// Update it
	workingSet.Name = "Updated Name"
	workingSet.Servers = ServerList{
		{Type: "image", Image: "docker/updated:latest"},
	}
	workingSet.Secrets = SecretMap{
		"secret2": {Provider: "docker-desktop-store"},
	}

	err = dao.UpdateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	// Verify the update
	retrieved, err := dao.GetWorkingSet(ctx, "update-test")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", retrieved.Name)
	assert.Equal(t, workingSet.Servers, retrieved.Servers)
	assert.Equal(t, workingSet.Secrets, retrieved.Secrets)
}

func TestUpdateWorkingSetNonexistent(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSet := WorkingSet{
		ID:      "nonexistent",
		Name:    "Test",
		Servers: ServerList{},
		Secrets: SecretMap{},
	}

	// Should not error, but should not affect any rows
	err := dao.UpdateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	// Verify it wasn't created
	_, err = dao.GetWorkingSet(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestRemoveWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSet := WorkingSet{
		ID:      "remove-test",
		Name:    "To Remove",
		Servers: ServerList{},
		Secrets: SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	// Verify it exists
	_, err = dao.GetWorkingSet(ctx, "remove-test")
	require.NoError(t, err)

	// Remove it
	err = dao.RemoveWorkingSet(ctx, "remove-test")
	require.NoError(t, err)

	// Verify it's gone
	_, err = dao.GetWorkingSet(ctx, "remove-test")
	assert.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestRemoveWorkingSetNonexistent(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Should not error even if it doesn't exist
	err := dao.RemoveWorkingSet(ctx, "nonexistent")
	require.NoError(t, err)
}

func TestListWorkingSets(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create multiple working sets
	workingSets := []WorkingSet{
		{
			ID:      "list-1",
			Name:    "First",
			Servers: ServerList{},
			Secrets: SecretMap{},
		},
		{
			ID:      "list-2",
			Name:    "Second",
			Servers: ServerList{},
			Secrets: SecretMap{},
		},
		{
			ID:      "list-3",
			Name:    "Third",
			Servers: ServerList{},
			Secrets: SecretMap{},
		},
	}

	for _, ws := range workingSets {
		err := dao.CreateWorkingSet(ctx, ws)
		require.NoError(t, err)
	}

	// List them
	retrieved, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Len(t, retrieved, 3)

	// Check that all IDs are present
	ids := make(map[string]bool)
	for _, ws := range retrieved {
		ids[ws.ID] = true
	}
	assert.True(t, ids["list-1"])
	assert.True(t, ids["list-2"])
	assert.True(t, ids["list-3"])
}

func TestListWorkingSetsEmpty(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	retrieved, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Len(t, retrieved, 0)
}

func TestFindWorkingSetsByIDPrefix(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create working sets with different prefixes
	workingSets := []WorkingSet{
		{ID: "dev-frontend", Name: "Dev Frontend", Servers: ServerList{}, Secrets: SecretMap{}},
		{ID: "dev-backend", Name: "Dev Backend", Servers: ServerList{}, Secrets: SecretMap{}},
		{ID: "prod-frontend", Name: "Prod Frontend", Servers: ServerList{}, Secrets: SecretMap{}},
		{ID: "prod-backend", Name: "Prod Backend", Servers: ServerList{}, Secrets: SecretMap{}},
		{ID: "test-env", Name: "Test Env", Servers: ServerList{}, Secrets: SecretMap{}},
	}

	for _, ws := range workingSets {
		err := dao.CreateWorkingSet(ctx, ws)
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		prefix        string
		expectedCount int
		expectedIDs   []string
	}{
		{
			name:          "dev prefix",
			prefix:        "dev",
			expectedCount: 2,
			expectedIDs:   []string{"dev-frontend", "dev-backend"},
		},
		{
			name:          "prod prefix",
			prefix:        "prod",
			expectedCount: 2,
			expectedIDs:   []string{"prod-frontend", "prod-backend"},
		},
		{
			name:          "test prefix",
			prefix:        "test",
			expectedCount: 1,
			expectedIDs:   []string{"test-env"},
		},
		{
			name:          "dev-f prefix (more specific)",
			prefix:        "dev-f",
			expectedCount: 1,
			expectedIDs:   []string{"dev-frontend"},
		},
		{
			name:          "nonexistent prefix",
			prefix:        "staging",
			expectedCount: 0,
			expectedIDs:   []string{},
		},
		{
			name:          "empty prefix",
			prefix:        "",
			expectedCount: 5,
			expectedIDs:   []string{"dev-frontend", "dev-backend", "prod-frontend", "prod-backend", "test-env"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrieved, err := dao.FindWorkingSetsByIDPrefix(ctx, tt.prefix)
			require.NoError(t, err)
			expectedCount := len(tt.expectedIDs)
			assert.Len(t, retrieved, expectedCount)

			if expectedCount > 0 {
				ids := make(map[string]bool)
				for _, ws := range retrieved {
					ids[ws.ID] = true
				}
				for _, expectedID := range tt.expectedIDs {
					assert.True(t, ids[expectedID], "Expected ID %s not found", expectedID)
				}
			}
		})
	}
}

func TestServerListJSONMarshaling(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSet := WorkingSet{
		ID:   "json-test",
		Name: "JSON Test",
		Servers: ServerList{
			{
				Type:    "registry",
				Source:  "https://example.com/server",
				Config:  map[string]any{"timeout": 30, "retry": true},
				Secrets: "default",
				Tools:   []string{"tool1", "tool2"},
			},
			{
				Type:  "image",
				Image: "docker/test:latest",
			},
		},
		Secrets: SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	retrieved, err := dao.GetWorkingSet(ctx, "json-test")
	require.NoError(t, err)

	// Verify servers were properly marshaled/unmarshaled
	assert.Len(t, retrieved.Servers, 2)

	// Check first server
	assert.Equal(t, "registry", retrieved.Servers[0].Type)
	assert.Equal(t, "https://example.com/server", retrieved.Servers[0].Source)
	assert.Equal(t, "default", retrieved.Servers[0].Secrets)
	assert.Equal(t, []string{"tool1", "tool2"}, retrieved.Servers[0].Tools)
	assert.Equal(t, map[string]any{"timeout": float64(30), "retry": true}, retrieved.Servers[0].Config)

	// Check second server
	assert.Equal(t, "image", retrieved.Servers[1].Type)
	assert.Equal(t, "docker/test:latest", retrieved.Servers[1].Image)
}

func TestSecretMapJSONMarshaling(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSet := WorkingSet{
		ID:      "secrets-test",
		Name:    "Secrets Test",
		Servers: ServerList{},
		Secrets: SecretMap{
			"default": {Provider: "docker-desktop-store"},
			"custom":  {Provider: "custom-provider"},
		},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	retrieved, err := dao.GetWorkingSet(ctx, "secrets-test")
	require.NoError(t, err)

	// Verify secrets were properly marshaled/unmarshaled
	assert.Len(t, retrieved.Secrets, 2)
	assert.Equal(t, "docker-desktop-store", retrieved.Secrets["default"].Provider)
	assert.Equal(t, "custom-provider", retrieved.Secrets["custom"].Provider)
}

func TestComplexWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Test a complex working set with multiple servers and secrets
	workingSet := WorkingSet{
		ID:   "complex-test",
		Name: "Complex Working Set",
		Servers: ServerList{
			{
				Type:   "registry",
				Source: "https://registry1.example.com",
				Config: map[string]any{
					"timeout":   60,
					"retries":   3,
					"endpoints": []any{"api1", "api2", "api3"},
				},
				Secrets: "registry-secrets",
				Tools:   []string{"fetch", "update"},
			},
			{
				Type:  "image",
				Image: "docker/server1:v1.0",
			},
			{
				Type:   "registry",
				Source: "https://registry2.example.com",
				Config: map[string]any{
					"nested": map[string]any{
						"key1": "value1",
						"key2": 123,
					},
				},
				Tools: []string{"deploy", "rollback"},
			},
		},
		Secrets: SecretMap{
			"registry-secrets": {Provider: "docker-desktop-store"},
			"db-secrets":       {Provider: "custom-vault"},
			"api-keys":         {Provider: "env-vars"},
		},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	retrieved, err := dao.GetWorkingSet(ctx, "complex-test")
	require.NoError(t, err)

	// Verify all data was preserved
	assert.Equal(t, workingSet.ID, retrieved.ID)
	assert.Equal(t, workingSet.Name, retrieved.Name)
	assert.Len(t, retrieved.Servers, 3)
	assert.Len(t, retrieved.Secrets, 3)

	// Verify complex nested structures
	assert.Equal(t, "registry", retrieved.Servers[0].Type)
	config := retrieved.Servers[0].Config
	assert.Equal(t, float64(60), config["timeout"])
	assert.Equal(t, float64(3), config["retries"])
	endpoints, ok := config["endpoints"].([]any)
	require.True(t, ok)
	assert.Len(t, endpoints, 3)
}

func TestConcurrentOperations(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create initial working set
	workingSet := WorkingSet{
		ID:      "concurrent-test",
		Name:    "Concurrent Test",
		Servers: ServerList{},
		Secrets: SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, workingSet)
	require.NoError(t, err)

	// Perform multiple reads concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := dao.GetWorkingSet(ctx, "concurrent-test")
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
