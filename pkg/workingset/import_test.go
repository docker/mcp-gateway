package workingset

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/db"
)

func TestImportYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a YAML file
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "import.yaml")

	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-import",
		Name:    "Imported Working Set",
		Servers: []Server{
			{
				Type:   ServerTypeRegistry,
				Source: "https://example.com/server",
				Config: map[string]any{"key": "value"},
				Tools:  []string{"tool1"},
			},
		},
		Secrets: map[string]Secret{
			"default": {Provider: SecretProviderDockerDesktop},
		},
	}

	data, err := yaml.Marshal(workingSet)
	require.NoError(t, err)
	err = os.WriteFile(yamlFile, data, 0o644)
	require.NoError(t, err)

	// Import the file
	err = Import(ctx, dao, yamlFile)
	require.NoError(t, err)

	// Verify it was imported
	dbSet, err := dao.GetWorkingSet(ctx, "test-import")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Equal(t, "test-import", dbSet.ID)
	assert.Equal(t, "Imported Working Set", dbSet.Name)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "registry", dbSet.Servers[0].Type)
}

func TestImportJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a JSON file
	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "import.json")

	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-import",
		Name:    "Imported Working Set",
		Servers: []Server{
			{
				Type:  ServerTypeImage,
				Image: "docker/test:latest",
			},
		},
		Secrets: map[string]Secret{
			"default": {Provider: SecretProviderDockerDesktop},
		},
	}

	data, err := json.MarshalIndent(workingSet, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(jsonFile, data, 0o644)
	require.NoError(t, err)

	// Import the file
	err = Import(ctx, dao, jsonFile)
	require.NoError(t, err)

	// Verify it was imported
	dbSet, err := dao.GetWorkingSet(ctx, "test-import")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Equal(t, "test-import", dbSet.ID)
	assert.Equal(t, "Imported Working Set", dbSet.Name)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "image", dbSet.Servers[0].Type)
}

func TestImportCreatesNewWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a YAML file
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "import.yaml")

	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "new-set",
		Name:    "New Working Set",
		Servers: []Server{},
		Secrets: map[string]Secret{},
	}

	data, err := yaml.Marshal(workingSet)
	require.NoError(t, err)
	err = os.WriteFile(yamlFile, data, 0o644)
	require.NoError(t, err)

	// Verify set doesn't exist
	_, err = dao.GetWorkingSet(ctx, "new-set")
	require.ErrorIs(t, err, sql.ErrNoRows)

	// Import the file
	err = Import(ctx, dao, yamlFile)
	require.NoError(t, err)

	// Verify set was created
	dbSet, err := dao.GetWorkingSet(ctx, "new-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "new-set", dbSet.ID)
}

func TestImportUpdatesExistingWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create an existing working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "existing-set",
		Name: "Original Name",
		Servers: db.ServerList{
			{Type: "image", Image: "old:latest"},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Create an import file with updated data
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "import.yaml")

	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "existing-set",
		Name:    "Updated Name",
		Servers: []Server{
			{Type: ServerTypeImage, Image: "new:latest"},
		},
		Secrets: map[string]Secret{
			"default": {Provider: SecretProviderDockerDesktop},
		},
	}

	data, err := yaml.Marshal(workingSet)
	require.NoError(t, err)
	err = os.WriteFile(yamlFile, data, 0o644)
	require.NoError(t, err)

	// Import the file
	err = Import(ctx, dao, yamlFile)
	require.NoError(t, err)

	// Verify set was updated
	dbSet, err := dao.GetWorkingSet(ctx, "existing-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Equal(t, "existing-set", dbSet.ID)
	assert.Equal(t, "Updated Name", dbSet.Name)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "new:latest", dbSet.Servers[0].Image)
}

func TestImportInvalidFile(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Try to import non-existent file
	err := Import(ctx, dao, "/nonexistent/file.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

func TestImportInvalidYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create an invalid YAML file
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "invalid.yaml")

	err := os.WriteFile(yamlFile, []byte("invalid: yaml: content: ["), 0o644)
	require.NoError(t, err)

	// Try to import
	err = Import(ctx, dao, yamlFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestImportInvalidJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create an invalid JSON file
	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "invalid.json")

	err := os.WriteFile(jsonFile, []byte("{invalid json}"), 0o644)
	require.NoError(t, err)

	// Try to import
	err = Import(ctx, dao, jsonFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestImportUnsupportedExtension(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a file with unsupported extension
	tempDir := t.TempDir()
	txtFile := filepath.Join(tempDir, "import.txt")

	err := os.WriteFile(txtFile, []byte("some content"), 0o644)
	require.NoError(t, err)

	// Try to import
	err = Import(ctx, dao, txtFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported file extension")
}

func TestImportValidationFailure(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a YAML file with invalid data (missing required fields)
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "invalid.yaml")

	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		// Missing ID and Name
		Servers: []Server{},
		Secrets: map[string]Secret{},
	}

	data, err := yaml.Marshal(workingSet)
	require.NoError(t, err)
	err = os.WriteFile(yamlFile, data, 0o644)
	require.NoError(t, err)

	// Try to import
	err = Import(ctx, dao, yamlFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid working set")
}

func TestImportEmptyFile(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create an empty YAML file
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "empty.yaml")

	err := os.WriteFile(yamlFile, []byte(""), 0o644)
	require.NoError(t, err)

	// Try to import
	err = Import(ctx, dao, yamlFile)
	require.Error(t, err)
	// Empty file will fail validation
}
