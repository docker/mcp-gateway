package workingset

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
)

func TestUpdateConfig_SetSingleValue(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with a server that has a snapshot
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{"test-server.api_key=secret123"}, []string{}, []string{}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "test-server.api_key=secret123")

	// Verify the config was updated in the database
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Equal(t, "secret123", dbSet.Servers[0].Config["api_key"])
}

func TestUpdateConfig_SetMultipleValues(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with a server
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{
			"test-server.api_key=secret123",
			"test-server.timeout=30",
			"test-server.enabled=true",
		}, []string{}, []string{}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "test-server.api_key=secret123")
	assert.Contains(t, output, "test-server.timeout=30")
	assert.Contains(t, output, "test-server.enabled=true")

	// Verify all configs were updated
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Equal(t, "secret123", dbSet.Servers[0].Config["api_key"])
	assert.Equal(t, "30", dbSet.Servers[0].Config["timeout"])
	assert.Equal(t, "true", dbSet.Servers[0].Config["enabled"])
}

func TestUpdateConfig_GetSingleValue(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with config values
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{"test-server.api_key"}, []string{}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "test-server.api_key=secret123")
}

func TestUpdateConfig_GetMultipleValues(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with config values
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
					"enabled": true,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{
			"test-server.api_key",
			"test-server.timeout",
		}, []string{}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "test-server.api_key=secret123")
	assert.Contains(t, output, "test-server.timeout=30")
}

func TestUpdateConfig_GetAll(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with multiple servers
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "server1",
					},
				},
			},
			{
				Type:  "image",
				Image: "anotherimage:v1.0",
				Config: map[string]any{
					"host": "localhost",
					"port": 8080,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "server2",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{}, []string{}, true, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "server1.api_key=secret123")
	assert.Contains(t, output, "server1.timeout=30")
	assert.Contains(t, output, "server2.host=localhost")
	assert.Contains(t, output, "server2.port=8080")
}

func TestUpdateConfig_SetAndGet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"existing_key": "existing_value",
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set",
			[]string{"test-server.new_key=new_value"},
			[]string{"test-server.existing_key"},
			[]string{},
			false,
			OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "test-server.new_key=new_value")
	assert.Contains(t, output, "test-server.existing_key=existing_value")

	// Verify the new config was saved
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Equal(t, "new_value", dbSet.Servers[0].Config["new_key"])
	assert.Equal(t, "existing_value", dbSet.Servers[0].Config["existing_key"])
}

func TestUpdateConfig_JSONOutput(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{"test-server.api_key"}, []string{}, false, OutputFormatJSON)
		require.NoError(t, err)
	})

	var result map[string]string
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)
	assert.Equal(t, "secret123", result["test-server.api_key"])
}

func TestUpdateConfig_YAMLOutput(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{"test-server.api_key"}, []string{}, false, OutputFormatYAML)
		require.NoError(t, err)
	})

	var result map[string]string
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)
	assert.Equal(t, "secret123", result["test-server.api_key"])
}

func TestUpdateConfig_WorkingSetNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	ociService := getMockOciService()

	err := UpdateConfig(ctx, dao, ociService, "non-existent", []string{}, []string{}, []string{}, false, OutputFormatHumanReadable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateConfig_ServerNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	err = UpdateConfig(ctx, dao, ociService, "test-set", []string{"non-existent.key=value"}, []string{}, []string{}, false, OutputFormatHumanReadable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server non-existent not found")
}

func TestUpdateConfig_InvalidSetFormat_NoEquals(t *testing.T) {
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

	ociService := getMockOciService()

	err = UpdateConfig(ctx, dao, ociService, "test-set", []string{"invalid-format"}, []string{}, []string{}, false, OutputFormatHumanReadable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config argument")
}

func TestUpdateConfig_InvalidSetFormat_NoDot(t *testing.T) {
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

	ociService := getMockOciService()

	err = UpdateConfig(ctx, dao, ociService, "test-set", []string{"invalidformat=value"}, []string{}, []string{}, false, OutputFormatHumanReadable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config argument")
}

func TestUpdateConfig_InvalidGetFormat(t *testing.T) {
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

	ociService := getMockOciService()

	err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{"invalidformat"}, []string{}, false, OutputFormatHumanReadable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config argument")
}

func TestUpdateConfig_GetNonExistentKey(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with a server but no config
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{"test-server.nonexistent"}, []string{}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	// Non-existent keys should not produce output
	assert.Empty(t, output)
}

func TestUpdateConfig_UpdateExistingValue(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with existing config
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "old_value",
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{"test-server.api_key=new_value"}, []string{}, []string{}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "test-server.api_key=new_value")

	// Verify the value was updated
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Equal(t, "new_value", dbSet.Servers[0].Config["api_key"])
}

func TestUpdateConfig_EmptyValue(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{"test-server.api_key="}, []string{}, []string{}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "test-server.api_key=")

	// Verify empty value was set
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers[0].Config["api_key"])
}

func TestUpdateConfig_UnsupportedOutputFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{}, []string{}, false, OutputFormat("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestUpdateConfig_DeleteSingleValue(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with config values
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{}, []string{"test-server.api_key"}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	// Deleting should not produce output
	assert.Empty(t, output)

	// Verify the config was deleted from the database
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Nil(t, dbSet.Servers[0].Config["api_key"])
	// But other config should remain
	assert.Equal(t, 30, int(dbSet.Servers[0].Config["timeout"].(float64)))
}

func TestUpdateConfig_DeleteMultipleValues(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with config values
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
					"enabled": true,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{}, []string{
			"test-server.api_key",
			"test-server.timeout",
		}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	// Deleting should not produce output
	assert.Empty(t, output)

	// Verify both configs were deleted from the database
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Nil(t, dbSet.Servers[0].Config["api_key"])
	assert.Nil(t, dbSet.Servers[0].Config["timeout"])
	// But other config should remain
	assert.Equal(t, true, dbSet.Servers[0].Config["enabled"])
}

func TestUpdateConfig_DeleteNonExistentKey(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with a server but no config
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{}, []string{"test-server.nonexistent"}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	// Deleting non-existent keys should not produce output
	assert.Empty(t, output)

	// Verify existing config is still there
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Equal(t, "secret123", dbSet.Servers[0].Config["api_key"])
}

func TestUpdateConfig_DeleteAndGet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with config values
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set",
			[]string{},
			[]string{"test-server.timeout"},
			[]string{"test-server.api_key"},
			false,
			OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	// Should only show the value that wasn't deleted
	assert.Contains(t, output, "test-server.timeout=30")
	assert.NotContains(t, output, "api_key")

	// Verify the api_key was deleted
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Nil(t, dbSet.Servers[0].Config["api_key"])
	assert.Equal(t, 30, int(dbSet.Servers[0].Config["timeout"].(float64)))
}

func TestUpdateConfig_DeleteInvalidFormat_NoDot(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{}, []string{"invalidformat"}, false, OutputFormatHumanReadable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config argument")
}

func TestUpdateConfig_DeleteServerNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{}, []string{"non-existent.key"}, false, OutputFormatHumanReadable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server non-existent not found")
}

func TestUpdateConfig_DeleteAndSetConflict(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	// Try to both delete and set the same key
	err = UpdateConfig(ctx, dao, ociService, "test-set",
		[]string{"test-server.api_key=new_value"},
		[]string{},
		[]string{"test-server.api_key"},
		false,
		OutputFormatHumanReadable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot both delete and set the same config value")
}

func TestUpdateConfig_DeleteAllConfigFromServer(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with multiple config values
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
					"enabled": true,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{}, []string{
			"test-server.api_key",
			"test-server.timeout",
			"test-server.enabled",
		}, false, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	// Deleting should not produce output
	assert.Empty(t, output)

	// Verify all configs were deleted
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Nil(t, dbSet.Servers[0].Config["api_key"])
	assert.Nil(t, dbSet.Servers[0].Config["timeout"])
	assert.Nil(t, dbSet.Servers[0].Config["enabled"])
}

func TestUpdateConfig_DeleteWithJSONOutput(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with config values
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{}, []string{"test-server.api_key"}, false, OutputFormatJSON)
		require.NoError(t, err)
	})

	// Should output empty JSON object
	var result map[string]string
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)
	assert.Empty(t, result)

	// Verify the config was deleted
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Nil(t, dbSet.Servers[0].Config["api_key"])
	assert.Equal(t, 30, int(dbSet.Servers[0].Config["timeout"].(float64)))
}

func TestUpdateConfig_DeleteWithYAMLOutput(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with config values
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Config: map[string]any{
					"api_key": "secret123",
					"timeout": 30,
				},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	ociService := getMockOciService()

	output := captureStdout(func() {
		err = UpdateConfig(ctx, dao, ociService, "test-set", []string{}, []string{}, []string{"test-server.api_key"}, false, OutputFormatYAML)
		require.NoError(t, err)
	})

	// Should output empty YAML object
	var result map[string]string
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)
	assert.Empty(t, result)

	// Verify the config was deleted
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Nil(t, dbSet.Servers[0].Config["api_key"])
	assert.Equal(t, 30, int(dbSet.Servers[0].Config["timeout"].(float64)))
}
