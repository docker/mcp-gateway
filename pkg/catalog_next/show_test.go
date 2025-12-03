package catalognext

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func TestShowNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Show(ctx, dao, getMockOciService(), "test/nonexistent:latest", workingset.OutputFormatJSON, PullOptionNever, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog test/nonexistent:latest not found")
}

func TestShowHumanReadable(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Ref:    "test/catalog:latest",
		Source: "test-source",
		CatalogArtifact: CatalogArtifact{
			Title: "test-catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/test:v1",
					Tools: []string{"tool1", "tool2"},
				},
				{
					Type:   workingset.ServerTypeRegistry,
					Source: "https://example.com/api",
					Tools:  []string{"tool3"},
				},
			},
		},
	}

	dbCat, err := catalog.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := Show(ctx, dao, getMockOciService(), catalog.Ref, workingset.OutputFormatHumanReadable, PullOptionNever, false)
		require.NoError(t, err)
	})

	// Verify human-readable format
	assert.Contains(t, output, "Reference: "+catalog.Ref)
	assert.Contains(t, output, "Title: test-catalog")
	assert.Contains(t, output, "Source: test-source")
	assert.Contains(t, output, "Servers:")
	assert.Contains(t, output, "Type: image")
	assert.Contains(t, output, "Image: docker/test:v1")
	assert.Contains(t, output, "Type: registry")
	assert.Contains(t, output, "Source: https://example.com/api")
}

func TestShowJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Ref:    "test/json-test:latest",
		Source: "json-source",
		CatalogArtifact: CatalogArtifact{
			Title: "json-test",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/test:json",
					Tools: []string{"read", "write"},
				},
			},
		},
	}

	dbCat, err := catalog.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := Show(ctx, dao, getMockOciService(), catalog.Ref, workingset.OutputFormatJSON, PullOptionNever, false)
		require.NoError(t, err)
	})

	// Parse and verify JSON
	var result CatalogWithDigest
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	digest, err := catalog.Digest()
	require.NoError(t, err)
	assert.Equal(t, digest, result.Digest)
	assert.Equal(t, "json-test", result.Title)
	assert.Equal(t, "json-source", result.Source)
	assert.Len(t, result.Servers, 1)
	assert.Equal(t, workingset.ServerTypeImage, result.Servers[0].Type)
	assert.Equal(t, "docker/test:json", result.Servers[0].Image)
	assert.Equal(t, []string{"read", "write"}, result.Servers[0].Tools)
}

func TestShowYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalog := Catalog{
		Ref:    "test/yaml-test:latest",
		Source: "yaml-source",
		CatalogArtifact: CatalogArtifact{
			Title: "yaml-test",
			Servers: []Server{
				{
					Type:   workingset.ServerTypeRegistry,
					Source: "https://yaml.example.com",
					Tools:  []string{"deploy"},
				},
			},
		},
	}

	dbCat, err := catalog.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := Show(ctx, dao, getMockOciService(), catalog.Ref, workingset.OutputFormatYAML, PullOptionNever, false)
		require.NoError(t, err)
	})

	// Parse and verify YAML
	var result CatalogWithDigest
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	digest, err := catalog.Digest()
	require.NoError(t, err)
	assert.Equal(t, digest, result.Digest)
	assert.Equal(t, "yaml-test", result.Title)
	assert.Equal(t, "yaml-source", result.Source)
	assert.Len(t, result.Servers, 1)
	assert.Equal(t, workingset.ServerTypeRegistry, result.Servers[0].Type)
	assert.Equal(t, "https://yaml.example.com", result.Servers[0].Source)
	assert.Equal(t, []string{"deploy"}, result.Servers[0].Tools)
}

func TestShowWithSnapshot(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	snapshot := &workingset.ServerSnapshot{
		Server: catalog.Server{
			Name:        "snapshot-server",
			Description: "A server with snapshot",
		},
	}

	catalogObj := Catalog{
		Ref: "test/snapshot-catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "snapshot-catalog",
			Servers: []Server{
				{
					Type:     workingset.ServerTypeImage,
					Image:    "docker/snapshot:v1",
					Snapshot: snapshot,
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := Show(ctx, dao, getMockOciService(), catalogObj.Ref, workingset.OutputFormatJSON, PullOptionNever, false)
		require.NoError(t, err)
	})

	var result CatalogWithDigest
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	require.NotNil(t, result.Servers[0].Snapshot)
	assert.Equal(t, "snapshot-server", result.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "A server with snapshot", result.Servers[0].Snapshot.Server.Description)
}

func TestShowInvalidReferenceWithDigest(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Show(ctx, dao, getMockOciService(), "test/invalid-reference@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1", workingset.OutputFormatJSON, PullOptionNever, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reference test/invalid-reference@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1 must be a valid OCI reference without a digest")
}

func TestShowWithNoTools(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	snapshot := &workingset.ServerSnapshot{
		Server: catalog.Server{
			Name:        "snapshot-server",
			Description: "A server with snapshot",
			Tools: []catalog.Tool{
				{Name: "snapshot-tool1", Description: "First snapshot tool"},
				{Name: "snapshot-tool2", Description: "Second snapshot tool"},
			},
		},
	}

	catalogObj := Catalog{
		Ref: "test/no-tools-catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "no-tools-catalog",
			Servers: []Server{
				{
					Type:     workingset.ServerTypeImage,
					Image:    "docker/test:v1",
					Tools:    []string{"tool1", "tool2", "tool3"},
					Snapshot: snapshot,
				},
				{
					Type:   workingset.ServerTypeRegistry,
					Source: "https://example.com/api",
					Tools:  []string{"tool4", "tool5"},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := Show(ctx, dao, getMockOciService(), catalogObj.Ref, workingset.OutputFormatJSON, PullOptionNever, true)
		require.NoError(t, err)
	})

	var result CatalogWithDigest
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Verify tools are filtered out
	assert.Len(t, result.Servers, 2)
	assert.Nil(t, result.Servers[0].Tools)
	assert.Nil(t, result.Servers[1].Tools)

	// Verify snapshot tools are also filtered out
	require.NotNil(t, result.Servers[0].Snapshot)
	assert.Nil(t, result.Servers[0].Snapshot.Server.Tools)
}

// TODO(cody): Add tests for pull once we have proper mocks in place
func TestInvalidPullOption(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Show(ctx, dao, getMockOciService(), "test/catalog:latest", workingset.OutputFormatJSON, "invalid", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse pull option invalid: should be missing, never, always, or duration (e.g. '1h', '1d')")
}
