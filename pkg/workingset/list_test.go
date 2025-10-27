package workingset

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// captureStdout captures stdout during function execution
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestListEmpty(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	output := captureStdout(func() {
		err := List(ctx, dao, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "No working sets found")
}

func TestListHumanReadable(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create some working sets
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "set-1",
		Name:    "First Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "set-2",
		Name:    "Second Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := List(ctx, dao, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	// Verify header
	assert.Contains(t, output, "ID\tName")
	assert.Contains(t, output, "----\t----")

	// Verify data
	assert.Contains(t, output, "set-1\tFirst Set")
	assert.Contains(t, output, "set-2\tSecond Set")
}

func TestListJSON(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create some working sets
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "First Set",
		Servers: db.ServerList{
			{Type: "image", Image: "test:latest"},
		},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := List(ctx, dao, OutputFormatJSON)
		require.NoError(t, err)
	})

	// Parse JSON output
	var workingSets []WorkingSet
	err = json.Unmarshal([]byte(output), &workingSets)
	require.NoError(t, err)

	assert.Len(t, workingSets, 1)
	assert.Equal(t, "set-1", workingSets[0].ID)
	assert.Equal(t, "First Set", workingSets[0].Name)
	assert.Len(t, workingSets[0].Servers, 1)
	assert.Equal(t, ServerTypeImage, workingSets[0].Servers[0].Type)
}

func TestListYAML(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create some working sets
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "First Set",
		Servers: db.ServerList{
			{Type: "registry", Source: "https://example.com/server"},
		},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := List(ctx, dao, OutputFormatYAML)
		require.NoError(t, err)
	})

	// Parse YAML output
	var workingSets []WorkingSet
	err = yaml.Unmarshal([]byte(output), &workingSets)
	require.NoError(t, err)

	assert.Len(t, workingSets, 1)
	assert.Equal(t, "set-1", workingSets[0].ID)
	assert.Equal(t, "First Set", workingSets[0].Name)
	assert.Len(t, workingSets[0].Servers, 1)
	assert.Equal(t, ServerTypeRegistry, workingSets[0].Servers[0].Type)
}

func TestListMultipleWorkingSets(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create multiple working sets
	for i := 1; i <= 5; i++ {
		err := dao.CreateWorkingSet(ctx, db.WorkingSet{
			ID:      string(rune('a'+i-1)) + "-set",
			Name:    "Set " + string(rune('0'+i)),
			Servers: db.ServerList{},
			Secrets: db.SecretMap{},
		})
		require.NoError(t, err)
	}

	output := captureStdout(func() {
		err := List(ctx, dao, OutputFormatJSON)
		require.NoError(t, err)
	})

	var workingSets []WorkingSet
	err := json.Unmarshal([]byte(output), &workingSets)
	require.NoError(t, err)

	assert.Len(t, workingSets, 5)
}

func TestListUnsupportedFormat(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create at least one working set so the format validation is reached
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = List(ctx, dao, OutputFormat("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestListWithComplexWorkingSets(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create a complex working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "complex-set",
		Name: "Complex Set",
		Servers: db.ServerList{
			{
				Type:   "registry",
				Source: "https://example.com/server",
				Config: map[string]interface{}{
					"key1": "value1",
					"key2": 123,
				},
				Secrets: "secret1",
				Tools:   []string{"tool1", "tool2"},
			},
			{
				Type:    "image",
				Image:   "docker/test:latest",
				Secrets: "secret2",
				Tools:   []string{"tool3"},
			},
		},
		Secrets: db.SecretMap{
			"secret1": {Provider: "docker-desktop-store"},
			"secret2": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := List(ctx, dao, OutputFormatJSON)
		require.NoError(t, err)
	})

	var workingSets []WorkingSet
	err = json.Unmarshal([]byte(output), &workingSets)
	require.NoError(t, err)

	assert.Len(t, workingSets, 1)
	assert.Equal(t, "complex-set", workingSets[0].ID)
	assert.Len(t, workingSets[0].Servers, 2)
	assert.Len(t, workingSets[0].Secrets, 2)
}

func TestListPreservesServerOrder(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create working set with multiple servers
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "ordered-set",
		Name: "Ordered Set",
		Servers: db.ServerList{
			{Type: "image", Image: "first:latest"},
			{Type: "registry", Source: "https://second.example.com"},
			{Type: "image", Image: "third:latest"},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := List(ctx, dao, OutputFormatJSON)
		require.NoError(t, err)
	})

	var workingSets []WorkingSet
	err = json.Unmarshal([]byte(output), &workingSets)
	require.NoError(t, err)

	require.Len(t, workingSets, 1)
	require.Len(t, workingSets[0].Servers, 3)

	assert.Equal(t, "first:latest", workingSets[0].Servers[0].Image)
	assert.Equal(t, "https://second.example.com", workingSets[0].Servers[1].Source)
	assert.Equal(t, "third:latest", workingSets[0].Servers[2].Image)
}

func TestListNameWithSpecialCharacters(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create working set with special characters in name
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "special-set",
		Name:    "Set with\tTab and\nNewline",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := List(ctx, dao, OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	// Verify the output contains the ID and name (even with special chars)
	assert.Contains(t, output, "special-set")
}

func TestListEmptyServersAndSecrets(t *testing.T) {
	dao, _ := setupTestDB(t)
	ctx := t.Context()

	// Create working set with empty servers and secrets
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "empty-content",
		Name:    "Empty Content Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := List(ctx, dao, OutputFormatJSON)
		require.NoError(t, err)
	})

	var workingSets []WorkingSet
	err = json.Unmarshal([]byte(output), &workingSets)
	require.NoError(t, err)

	require.Len(t, workingSets, 1)
	assert.Len(t, workingSets[0].Servers, 0)
	assert.Len(t, workingSets[0].Secrets, 0)
}
