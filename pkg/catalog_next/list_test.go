package catalognext

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestListEmpty(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	output := captureStdout(t, func() {
		err := List(ctx, dao, workingset.OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "No catalogs found")
	assert.Contains(t, output, "docker mcp catalog-next create")
}

func TestListHumanReadable(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create test catalogs
	catalog1 := Catalog{
		Name: "catalog-one",
		Servers: []Server{
			{Type: workingset.ServerTypeImage, Image: "test:v1"},
		},
	}
	catalog2 := Catalog{
		Name: "catalog-two",
		Servers: []Server{
			{Type: workingset.ServerTypeImage, Image: "test:v2"},
		},
	}

	err := dao.CreateCatalog(ctx, catalog1.ToDb())
	require.NoError(t, err)
	err = dao.CreateCatalog(ctx, catalog2.ToDb())
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := List(ctx, dao, workingset.OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	// Verify table format
	assert.Contains(t, output, "Digest\tName")
	assert.Contains(t, output, "----\t----")
	assert.Contains(t, output, catalog1.Digest())
	assert.Contains(t, output, "catalog-one")
	assert.Contains(t, output, catalog2.Digest())
	assert.Contains(t, output, "catalog-two")
}

func TestListHumanReadableFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Name: "test-catalog",
		Servers: []Server{
			{Type: workingset.ServerTypeImage, Image: "test:latest"},
		},
	}

	err := dao.CreateCatalog(ctx, catalog.ToDb())
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := List(ctx, dao, workingset.OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 3) // header, separator, data

	// Verify header
	assert.Equal(t, "Digest\tName", lines[0])
	assert.Equal(t, "----\t----", lines[1])

	// Verify data line contains digest and name
	assert.Contains(t, lines[2], catalog.Digest())
	assert.Contains(t, lines[2], "test-catalog")
}

func TestListJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog1 := Catalog{
		Name:   "catalog-one",
		Source: "source-1",
		Servers: []Server{
			{
				Type:  workingset.ServerTypeImage,
				Image: "test:v1",
				Tools: []string{"tool1"},
			},
		},
	}
	catalog2 := Catalog{
		Name:   "catalog-two",
		Source: "source-2",
		Servers: []Server{
			{
				Type:   workingset.ServerTypeRegistry,
				Source: "https://example.com",
				Tools:  []string{"tool2", "tool3"},
			},
		},
	}

	err := dao.CreateCatalog(ctx, catalog1.ToDb())
	require.NoError(t, err)
	err = dao.CreateCatalog(ctx, catalog2.ToDb())
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := List(ctx, dao, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	// Parse JSON output
	var catalogs []CatalogWithDigest
	err = json.Unmarshal([]byte(output), &catalogs)
	require.NoError(t, err)
	assert.Len(t, catalogs, 2)

	// Verify first catalog
	assert.Equal(t, catalog1.Digest(), catalogs[0].Digest)
	assert.Equal(t, "catalog-one", catalogs[0].Name)
	assert.Equal(t, "source-1", catalogs[0].Source)
	assert.Len(t, catalogs[0].Servers, 1)
	assert.Equal(t, workingset.ServerTypeImage, catalogs[0].Servers[0].Type)
	assert.Equal(t, "test:v1", catalogs[0].Servers[0].Image)
	assert.Equal(t, []string{"tool1"}, catalogs[0].Servers[0].Tools)

	// Verify second catalog
	assert.Equal(t, catalog2.Digest(), catalogs[1].Digest)
	assert.Equal(t, "catalog-two", catalogs[1].Name)
	assert.Equal(t, "source-2", catalogs[1].Source)
	assert.Len(t, catalogs[1].Servers, 1)
	assert.Equal(t, workingset.ServerTypeRegistry, catalogs[1].Servers[0].Type)
	assert.Equal(t, "https://example.com", catalogs[1].Servers[0].Source)
	assert.Equal(t, []string{"tool2", "tool3"}, catalogs[1].Servers[0].Tools)
}

func TestListJSONEmpty(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	output := captureStdout(t, func() {
		err := List(ctx, dao, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	// Parse JSON output
	var catalogs []CatalogWithDigest
	err := json.Unmarshal([]byte(output), &catalogs)
	require.NoError(t, err)
	assert.Empty(t, catalogs)
}

func TestListYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog1 := Catalog{
		Name:   "catalog-yaml",
		Source: "yaml-source",
		Servers: []Server{
			{
				Type:  workingset.ServerTypeImage,
				Image: "test:yaml",
				Tools: []string{"tool1", "tool2"},
			},
		},
	}

	err := dao.CreateCatalog(ctx, catalog1.ToDb())
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := List(ctx, dao, workingset.OutputFormatYAML)
		require.NoError(t, err)
	})

	// Parse YAML output
	var catalogs []CatalogWithDigest
	err = yaml.Unmarshal([]byte(output), &catalogs)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	// Verify catalog
	assert.Equal(t, catalog1.Digest(), catalogs[0].Digest)
	assert.Equal(t, "catalog-yaml", catalogs[0].Name)
	assert.Equal(t, "yaml-source", catalogs[0].Source)
	assert.Len(t, catalogs[0].Servers, 1)
	assert.Equal(t, workingset.ServerTypeImage, catalogs[0].Servers[0].Type)
	assert.Equal(t, "test:yaml", catalogs[0].Servers[0].Image)
	assert.Equal(t, []string{"tool1", "tool2"}, catalogs[0].Servers[0].Tools)
}

func TestListYAMLEmpty(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	output := captureStdout(t, func() {
		err := List(ctx, dao, workingset.OutputFormatYAML)
		require.NoError(t, err)
	})

	// Parse YAML output
	var catalogs []CatalogWithDigest
	err := yaml.Unmarshal([]byte(output), &catalogs)
	require.NoError(t, err)
	assert.Empty(t, catalogs)
}

func TestListWithSnapshot(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	snapshot := &workingset.ServerSnapshot{
		Server: catalog.Server{
			Name:        "test-server",
			Description: "Test description",
		},
	}

	catalog := Catalog{
		Name: "snapshot-catalog",
		Servers: []Server{
			{
				Type:     workingset.ServerTypeImage,
				Image:    "test:snapshot",
				Snapshot: snapshot,
			},
		},
	}

	err := dao.CreateCatalog(ctx, catalog.ToDb())
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := List(ctx, dao, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var result []CatalogWithDigest
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)

	// Verify snapshot is included
	require.NotNil(t, result[0].Servers[0].Snapshot)
	assert.Equal(t, "test-server", result[0].Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "Test description", result[0].Servers[0].Snapshot.Server.Description)
}

func TestListWithMultipleServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Name: "multi-server-catalog",
		Servers: []Server{
			{
				Type:  workingset.ServerTypeImage,
				Image: "test:v1",
				Tools: []string{"tool1"},
			},
			{
				Type:   workingset.ServerTypeRegistry,
				Source: "https://example.com",
				Tools:  []string{"tool2"},
			},
			{
				Type:  workingset.ServerTypeImage,
				Image: "test:v2",
			},
		},
	}

	err := dao.CreateCatalog(ctx, catalog.ToDb())
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := List(ctx, dao, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var result []CatalogWithDigest
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Len(t, result[0].Servers, 3)

	// Just verify that all three server types are present, order may vary
	types := make(map[workingset.ServerType]int)
	for _, s := range result[0].Servers {
		types[s.Type]++
	}
	assert.Equal(t, 2, types[workingset.ServerTypeImage])
	assert.Equal(t, 1, types[workingset.ServerTypeRegistry])
}

func TestListHumanReadableEmptyDoesNotShowInJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// For JSON/YAML formats, empty list should return valid empty array
	outputJSON := captureStdout(t, func() {
		err := List(ctx, dao, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var catalogs []CatalogWithDigest
	err := json.Unmarshal([]byte(outputJSON), &catalogs)
	require.NoError(t, err)
	assert.Empty(t, catalogs)
	assert.NotContains(t, outputJSON, "No catalogs found")
}
