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

func TestShowHumanReadable(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:    "registry",
				Source:  "https://example.com/server",
				Config:  map[string]any{"key": "value"},
				Secrets: "default",
				Tools:   []string{"tool1", "tool2"},
			},
		},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Show(ctx, dao, "test-set", OutputFormatHumanReadable, false)
		require.NoError(t, err)
	})

	// Verify output contains key information
	assert.Contains(t, output, "ID: test-set")
	assert.Contains(t, output, "Name: Test Working Set")
	assert.Contains(t, output, "Type: registry")
	assert.Contains(t, output, "Source: https://example.com/server")
	assert.Contains(t, output, "Secrets: default")
	assert.Contains(t, output, "Provider: docker-desktop-store")
}

func TestShowJSON(t *testing.T) {
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

	output := captureStdout(func() {
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false)
		require.NoError(t, err)
	})

	// Parse JSON output
	var workingSet WorkingSet
	err = json.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Equal(t, "test-set", workingSet.ID)
	assert.Equal(t, "Test Working Set", workingSet.Name)
	assert.Len(t, workingSet.Servers, 1)
	assert.Equal(t, ServerTypeImage, workingSet.Servers[0].Type)
	assert.Equal(t, "docker/test:latest", workingSet.Servers[0].Image)
}

func TestShowYAML(t *testing.T) {
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
			},
		},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Show(ctx, dao, "test-set", OutputFormatYAML, false)
		require.NoError(t, err)
	})

	// Parse YAML output
	var workingSet WorkingSet
	err = yaml.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Equal(t, "test-set", workingSet.ID)
	assert.Equal(t, "Test Working Set", workingSet.Name)
	assert.Len(t, workingSet.Servers, 1)
	assert.Equal(t, ServerTypeRegistry, workingSet.Servers[0].Type)
}

func TestShowNonExistentWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Show(ctx, dao, "non-existent", OutputFormatJSON, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestShowUnsupportedFormat(t *testing.T) {
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

	err = Show(ctx, dao, "test-set", OutputFormat("invalid"), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestShowComplexWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a complex working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "complex-set",
		Name: "Complex Working Set",
		Servers: db.ServerList{
			{
				Type:   "registry",
				Source: "https://example.com/server1",
				Config: map[string]any{
					"key1": "value1",
					"key2": 123,
					"nested": map[string]any{
						"key": "value",
					},
				},
				Secrets: "secret1",
				Tools:   []string{"tool1", "tool2", "tool3"},
			},
			{
				Type:    "image",
				Image:   "docker/test:latest",
				Secrets: "secret2",
				Tools:   []string{"tool4"},
			},
		},
		Secrets: db.SecretMap{
			"secret1": {Provider: "docker-desktop-store"},
			"secret2": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Show(ctx, dao, "complex-set", OutputFormatJSON, false)
		require.NoError(t, err)
	})

	// Parse and verify complex data
	var workingSet WorkingSet
	err = json.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Len(t, workingSet.Servers, 2)
	assert.Len(t, workingSet.Secrets, 2)

	// Verify server 1
	assert.Equal(t, ServerTypeRegistry, workingSet.Servers[0].Type)
	assert.Equal(t, "secret1", workingSet.Servers[0].Secrets)
	assert.Len(t, workingSet.Servers[0].Tools, 3)
	assert.Contains(t, workingSet.Servers[0].Config, "key1")

	// Verify server 2
	assert.Equal(t, ServerTypeImage, workingSet.Servers[1].Type)
	assert.Equal(t, "secret2", workingSet.Servers[1].Secrets)
	assert.Len(t, workingSet.Servers[1].Tools, 1)
}

func TestShowEmptyWorkingSet(t *testing.T) {
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

	output := captureStdout(func() {
		err := Show(ctx, dao, "empty-set", OutputFormatJSON, false)
		require.NoError(t, err)
	})

	var workingSet WorkingSet
	err = json.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Equal(t, "empty-set", workingSet.ID)
	assert.Empty(t, workingSet.Servers)
	assert.Empty(t, workingSet.Secrets)
}

func TestPrintHumanReadableWithImageServer(t *testing.T) {
	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-id",
		Name:    "Test Working Set",
		Servers: []Server{
			{
				Type:    ServerTypeImage,
				Image:   "docker/test:latest",
				Config:  map[string]any{},
				Secrets: "default",
				Tools:   []string{"tool1"},
			},
		},
		Secrets: map[string]Secret{
			"default": {Provider: SecretProviderDockerDesktop},
		},
	}

	output := printHumanReadable(workingSet)

	assert.Contains(t, output, "Type: image")
	assert.Contains(t, output, "Image: docker/test:latest")
}

func TestPrintHumanReadableMultipleServers(t *testing.T) {
	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-id",
		Name:    "Test Working Set",
		Servers: []Server{
			{
				Type:   ServerTypeRegistry,
				Source: "https://example.com/server1",
			},
			{
				Type:  ServerTypeImage,
				Image: "docker/test:latest",
			},
		},
		Secrets: map[string]Secret{},
	}

	output := printHumanReadable(workingSet)

	// Verify both servers are in output
	assert.Contains(t, output, "https://example.com/server1")
	assert.Contains(t, output, "docker/test:latest")
}

func TestPrintHumanReadableMultipleSecrets(t *testing.T) {
	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-id",
		Name:    "Test Working Set",
		Servers: []Server{},
		Secrets: map[string]Secret{
			"secret1": {Provider: SecretProviderDockerDesktop},
			"secret2": {Provider: SecretProviderDockerDesktop},
		},
	}

	output := printHumanReadable(workingSet)

	// Verify both secrets are in output
	assert.Contains(t, output, "Name: secret1")
	assert.Contains(t, output, "Name: secret2")
}

func TestShowPreservesVersion(t *testing.T) {
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

	output := captureStdout(func() {
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false)
		require.NoError(t, err)
	})

	var workingSet WorkingSet
	err = json.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Equal(t, CurrentWorkingSetVersion, workingSet.Version)
}

func TestShowWithNilConfig(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with nil config
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:   "registry",
				Source: "https://example.com/server",
				Config: nil,
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false)
		require.NoError(t, err)
	})

	var workingSet WorkingSet
	err = json.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Len(t, workingSet.Servers, 1)
}

func TestShowWithEmptyTools(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with empty tools (all tools disabled)
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "docker/test:latest",
				Tools: []string{},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false)
		require.NoError(t, err)
	})

	var workingSet WorkingSet
	err = json.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Len(t, workingSet.Servers, 1)
	assert.NotNil(t, workingSet.Servers[0].Tools)
	assert.Empty(t, workingSet.Servers[0].Tools)
	assert.Contains(t, output, `"tools": []`)
}

func TestShowWithNilTools(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "docker/test:latest",
				Tools: nil,
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Show(ctx, dao, "test-set", OutputFormatJSON)
		require.NoError(t, err)
	})

	var workingSet WorkingSet
	err = json.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Len(t, workingSet.Servers, 1)
	assert.Nil(t, workingSet.Servers[0].Tools)
	assert.Contains(t, output, `"tools": null`)
}

func TestShowSnapshotWithIcon(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "docker/test:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name:        "Test Server",
						Type:        "server",
						Image:       "docker/test:latest",
						Description: "Test server description",
						Title:       "Test Server Title",
						Icon:        "https://example.com/icon.png",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false)
		require.NoError(t, err)
	})

	var workingSet WorkingSet
	err = json.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Equal(t, "https://example.com/icon.png", workingSet.Servers[0].Snapshot.Server.Icon)
}

func TestShowWithClientsFlag(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:   "registry",
				Source: "https://example.com/server",
			},
		},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	t.Run("JSON with clients flag", func(t *testing.T) {
		output := captureStdout(func() {
			err := Show(ctx, dao, "test-set", OutputFormatJSON, true)
			require.NoError(t, err)
		})

		var result WithOptions
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err)

		assert.Equal(t, "test-set", result.ID)
		assert.NotNil(t, result.Clients)
	})

	t.Run("YAML with clients flag", func(t *testing.T) {
		output := captureStdout(func() {
			err := Show(ctx, dao, "test-set", OutputFormatYAML, true)
			require.NoError(t, err)
		})

		var result WithOptions
		err = yaml.Unmarshal([]byte(output), &result)
		require.NoError(t, err)

		assert.Equal(t, "test-set", result.ID)
		assert.NotNil(t, result.Clients)
	})
}
