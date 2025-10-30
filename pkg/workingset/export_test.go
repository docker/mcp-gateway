package workingset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/db"
)

func TestExportYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:   "registry",
				Source: "https://example.com/server",
				Config: map[string]any{"key": "value"},
				Tools:  []string{"tool1"},
			},
		},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	// Export to YAML
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "export.yaml")

	err = Export(ctx, dao, "test-set", yamlFile)
	require.NoError(t, err)

	// Verify file was created
	assert.FileExists(t, yamlFile)

	// Read and verify contents
	data, err := os.ReadFile(yamlFile)
	require.NoError(t, err)

	var exported WorkingSet
	err = yaml.Unmarshal(data, &exported)
	require.NoError(t, err)

	assert.Equal(t, "test-set", exported.ID)
	assert.Equal(t, "Test Working Set", exported.Name)
	assert.Len(t, exported.Servers, 1)
	assert.Equal(t, ServerTypeRegistry, exported.Servers[0].Type)
	assert.Equal(t, "https://example.com/server", exported.Servers[0].Source)
}

func TestExportJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "docker/test:latest",
			},
		},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	// Export to JSON
	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "export.json")

	err = Export(ctx, dao, "test-set", jsonFile)
	require.NoError(t, err)

	// Verify file was created
	assert.FileExists(t, jsonFile)

	// Read and verify contents
	data, err := os.ReadFile(jsonFile)
	require.NoError(t, err)

	var exported WorkingSet
	err = json.Unmarshal(data, &exported)
	require.NoError(t, err)

	assert.Equal(t, "test-set", exported.ID)
	assert.Equal(t, "Test Working Set", exported.Name)
	assert.Len(t, exported.Servers, 1)
	assert.Equal(t, ServerTypeImage, exported.Servers[0].Type)
	assert.Equal(t, "docker/test:latest", exported.Servers[0].Image)
}

func TestExportNonExistentWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "export.yaml")

	err := Export(ctx, dao, "non-existent", yamlFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExportUnsupportedExtension(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	tempDir := t.TempDir()
	txtFile := filepath.Join(tempDir, "export.txt")

	err = Export(ctx, dao, "test-set", txtFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported file extension")
}

func TestExportEmptyWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create an empty working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "empty-set",
		Name:    "Empty Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Export to YAML
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "empty.yaml")

	err = Export(ctx, dao, "empty-set", yamlFile)
	require.NoError(t, err)

	// Read and verify contents
	data, err := os.ReadFile(yamlFile)
	require.NoError(t, err)

	var exported WorkingSet
	err = yaml.Unmarshal(data, &exported)
	require.NoError(t, err)

	assert.Equal(t, "empty-set", exported.ID)
	assert.Empty(t, exported.Servers)
	assert.Empty(t, exported.Secrets)
}

func TestExportToNonexistentDirectory(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to export to a directory that doesn't exist
	yamlFile := filepath.Join("/nonexistent", "path", "export.yaml")

	err = Export(ctx, dao, "test-set", yamlFile)
	require.Error(t, err)
}

func TestExportPreservesDataTypes(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with various data types in config
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:   "registry",
				Source: "https://example.com/server",
				Config: map[string]any{
					"string": "value",
					"int":    42,
					"float":  3.14,
					"bool":   true,
					"array":  []any{1, 2, 3},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Export to JSON (better for type preservation)
	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "export.json")

	err = Export(ctx, dao, "test-set", jsonFile)
	require.NoError(t, err)

	// Read and verify data types are preserved
	data, err := os.ReadFile(jsonFile)
	require.NoError(t, err)

	var exported WorkingSet
	err = json.Unmarshal(data, &exported)
	require.NoError(t, err)

	config := exported.Servers[0].Config
	assert.Equal(t, "value", config["string"])
	assert.IsType(t, float64(0), config["int"]) // JSON numbers are float64
	assert.InEpsilon(t, float64(42), config["int"], 0.0000001)
	assert.IsType(t, float64(0), config["float"])
	assert.InEpsilon(t, 3.14, config["float"], 0.0000001)
	assert.IsType(t, true, config["bool"])
	assert.Equal(t, true, config["bool"])
	assert.IsType(t, []any{}, config["array"])
}
