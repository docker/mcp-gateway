package workingset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateWithDockerImages(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, "", "My Test Set", []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	})
	require.NoError(t, err)

	// Verify the working set was created
	dbSet, err := dao.GetWorkingSet(ctx, "my-test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Equal(t, "my-test-set", dbSet.ID)
	assert.Equal(t, "My Test Set", dbSet.Name)
	assert.Len(t, dbSet.Servers, 2)

	assert.Equal(t, "image", dbSet.Servers[0].Type)
	assert.Equal(t, "myimage:latest", dbSet.Servers[0].Image)

	assert.Equal(t, "image", dbSet.Servers[1].Type)
	assert.Equal(t, "anotherimage:v1.0", dbSet.Servers[1].Image)
}

func TestCreateWithRegistryServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, "", "Registry Set", []string{
		"https://example.com/server1",
		"http://example.com/server2",
	})
	require.NoError(t, err)

	// Verify the working set was created
	dbSet, err := dao.GetWorkingSet(ctx, "registry-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Len(t, dbSet.Servers, 2)

	assert.Equal(t, "registry", dbSet.Servers[0].Type)
	assert.Equal(t, "https://example.com/server1", dbSet.Servers[0].Source)

	assert.Equal(t, "registry", dbSet.Servers[1].Type)
	assert.Equal(t, "http://example.com/server2", dbSet.Servers[1].Source)
}

func TestCreateWithMixedServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, "", "Mixed Set", []string{
		"docker://myimage:latest",
		"https://example.com/server",
	})
	require.NoError(t, err)

	// Verify the working set was created
	dbSet, err := dao.GetWorkingSet(ctx, "mixed-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Len(t, dbSet.Servers, 2)
	assert.Equal(t, "image", dbSet.Servers[0].Type)
	assert.Equal(t, "registry", dbSet.Servers[1].Type)
}

func TestCreateWithCustomId(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, "custom-id", "Test Set", []string{
		"docker://myimage:latest",
	})
	require.NoError(t, err)

	// Verify the working set was created with custom ID
	dbSet, err := dao.GetWorkingSet(ctx, "custom-id")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Equal(t, "custom-id", dbSet.ID)
	assert.Equal(t, "Test Set", dbSet.Name)
}

func TestCreateWithExistingId(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create first working set
	err := Create(ctx, dao, "test-id", "Test Set 1", []string{
		"docker://myimage:latest",
	})
	require.NoError(t, err)

	// Try to create another with the same ID
	err = Create(ctx, dao, "test-id", "Test Set 2", []string{
		"docker://anotherimage:latest",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCreateGeneratesUniqueIds(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create first working set
	err := Create(ctx, dao, "", "Test Set", []string{
		"docker://myimage:latest",
	})
	require.NoError(t, err)

	// Create second with same name
	err = Create(ctx, dao, "", "Test Set", []string{
		"docker://anotherimage:latest",
	})
	require.NoError(t, err)

	// Create third with same name
	err = Create(ctx, dao, "", "Test Set", []string{
		"docker://thirdimage:latest",
	})
	require.NoError(t, err)

	// List all working sets
	sets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Len(t, sets, 3)

	// Verify IDs are unique
	ids := make(map[string]bool)
	for _, set := range sets {
		assert.False(t, ids[set.ID], "ID %s should be unique", set.ID)
		ids[set.ID] = true
	}

	// Verify ID pattern
	assert.Contains(t, ids, "test-set")
	assert.Contains(t, ids, "test-set-2")
	assert.Contains(t, ids, "test-set-3")
}

func TestCreateWithInvalidServerFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, "", "Test Set", []string{
		"invalid-format",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid server value")
}

func TestCreateWithEmptyName(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, "test-id", "", []string{
		"docker://myimage:latest",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid working set")
}

func TestCreateWithEmptyServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, "", "Empty Set", []string{})
	require.NoError(t, err)

	// Verify the working set was created with no servers
	dbSet, err := dao.GetWorkingSet(ctx, "empty-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Empty(t, dbSet.Servers)
}

func TestCreateAddsDefaultSecrets(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, "", "Test Set", []string{
		"docker://myimage:latest",
	})
	require.NoError(t, err)

	// Verify default secrets were added
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Len(t, dbSet.Secrets, 1)
	assert.Contains(t, dbSet.Secrets, "default")
	assert.Equal(t, "docker-desktop-store", dbSet.Secrets["default"].Provider)
}

func TestCreateNameWithSpecialCharacters(t *testing.T) {
	tests := []struct {
		name       string
		inputName  string
		expectedID string
	}{
		{
			name:       "name with spaces",
			inputName:  "My Test Set",
			expectedID: "my-test-set",
		},
		{
			name:       "name with special chars",
			inputName:  "Test@Set#123!",
			expectedID: "test-set-123-",
		},
		{
			name:       "name with multiple spaces",
			inputName:  "Test   Set",
			expectedID: "test-set",
		},
		{
			name:       "name with underscores",
			inputName:  "Test_Set_Name",
			expectedID: "test-set-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh database for each subtest to avoid ID conflicts
			dao := setupTestDB(t)
			ctx := t.Context()

			err := Create(ctx, dao, "", tt.inputName, []string{
				"docker://myimage:latest",
			})
			require.NoError(t, err)

			// Verify the ID was generated correctly
			dbSet, err := dao.GetWorkingSet(ctx, tt.expectedID)
			require.NoError(t, err)
			require.NotNil(t, dbSet)
			assert.Equal(t, tt.expectedID, dbSet.ID)
		})
	}
}
