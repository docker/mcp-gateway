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
		err := Show(ctx, dao, "test-set", OutputFormatHumanReadable, false, "")
		require.NoError(t, err)
	})

	// Human readable is the same as yaml output
	// Parse YAML output
	var workingSet WorkingSet
	err = yaml.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Equal(t, "test-set", workingSet.ID)
	assert.Equal(t, "Test Working Set", workingSet.Name)
	assert.Len(t, workingSet.Servers, 1)
	assert.Equal(t, ServerTypeRegistry, workingSet.Servers[0].Type)
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
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false, "")
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
		err := Show(ctx, dao, "test-set", OutputFormatYAML, false, "")
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

func TestShowYAMLToolsShouldBeOmittedWhenAllEnabled(t *testing.T) {
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
		err := Show(ctx, dao, "test-set", OutputFormatYAML, false, "")
		require.NoError(t, err)
	})

	require.NotContains(t, output, "tools: []")
}

func TestShowYAMLToolsShouldBeEmptyArrayWhenAllDisabled(t *testing.T) {
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
				Tools:  []string{},
			},
		},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Show(ctx, dao, "test-set", OutputFormatYAML, false, "")
		require.NoError(t, err)
	})

	require.Contains(t, output, "tools: []")
}

func TestShowNonExistentWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Show(ctx, dao, "non-existent", OutputFormatJSON, false, "")
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

	err = Show(ctx, dao, "test-set", OutputFormat("invalid"), false, "")
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
		err := Show(ctx, dao, "complex-set", OutputFormatJSON, false, "")
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
		err := Show(ctx, dao, "empty-set", OutputFormatJSON, false, "")
		require.NoError(t, err)
	})

	var workingSet WorkingSet
	err = json.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Equal(t, "empty-set", workingSet.ID)
	assert.Empty(t, workingSet.Servers)
	assert.Empty(t, workingSet.Secrets)
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
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false, "")
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
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false, "")
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
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false, "")
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
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false, "")
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
		err := Show(ctx, dao, "test-set", OutputFormatJSON, false, "")
		require.NoError(t, err)
	})

	var workingSet WorkingSet
	err = json.Unmarshal([]byte(output), &workingSet)
	require.NoError(t, err)

	assert.Equal(t, "https://example.com/icon.png", workingSet.Servers[0].Snapshot.Server.Icon)
}

func TestShowWithYQExpressionNoTools(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "docker/test:v1",
				Tools: []string{"tool1", "tool2", "tool3"},
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name:        "snapshot-server",
						Description: "A server with snapshot",
						Tools: []catalog.Tool{
							{Name: "snapshot-tool1", Description: "First snapshot tool"},
							{Name: "snapshot-tool2", Description: "Second snapshot tool"},
						},
					},
				},
			},
			{
				Type:   "registry",
				Source: "https://example.com/api",
				Tools:  []string{"tool4", "tool5"},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	testCases := []struct {
		format OutputFormat
		parser func(t *testing.T, data []byte) WorkingSet
	}{
		{
			format: OutputFormatJSON,
			parser: func(t *testing.T, data []byte) WorkingSet {
				t.Helper()
				var result WorkingSet
				err := json.Unmarshal(data, &result)
				require.NoError(t, err)
				return result
			},
		},
		{
			format: OutputFormatYAML,
			parser: func(t *testing.T, data []byte) WorkingSet {
				t.Helper()
				var result WorkingSet
				err := yaml.Unmarshal(data, &result)
				require.NoError(t, err)
				return result
			},
		},
		{
			format: OutputFormatHumanReadable,
			parser: func(t *testing.T, data []byte) WorkingSet {
				t.Helper()
				var result WorkingSet
				err := yaml.Unmarshal(data, &result)
				require.NoError(t, err)
				return result
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.format), func(t *testing.T) {
			output := captureStdout(func() {
				err := Show(ctx, dao, "test-set", testCase.format, false, "del(.servers[].tools, .servers[].snapshot.server.tools)")
				require.NoError(t, err)
			})

			result := testCase.parser(t, []byte(output))

			// Verify tools are filtered out
			assert.Len(t, result.Servers, 2)
			assert.Nil(t, result.Servers[0].Tools)
			assert.Nil(t, result.Servers[1].Tools)

			// Verify snapshot tools are also filtered out
			require.NotNil(t, result.Servers[0].Snapshot)
			assert.Nil(t, result.Servers[0].Snapshot.Server.Tools)
		})
	}
}

func TestShowWithInvalidYQExpression(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = Show(ctx, dao, "test-set", OutputFormatJSON, false, ".invalid[")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to evaluate YQ expression")
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
			err := Show(ctx, dao, "test-set", OutputFormatJSON, true, "")
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
			err := Show(ctx, dao, "test-set", OutputFormatYAML, true, "")
			require.NoError(t, err)
		})

		var result WithOptions
		err = yaml.Unmarshal([]byte(output), &result)
		require.NoError(t, err)

		assert.Equal(t, "test-set", result.ID)
		assert.NotNil(t, result.Clients)
	})
}
