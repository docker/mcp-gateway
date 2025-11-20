package catalognext

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/docker/mcp-gateway/test/mocks"
)

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) db.DAO {
	t.Helper()

	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	dao, err := db.New(db.WithDatabaseFile(dbFile))
	require.NoError(t, err)

	return dao
}

func getMockOciService() oci.Service {
	return mocks.NewMockOCIService(mocks.WithLocalImages([]mocks.MockImage{
		{
			Ref: "myimage:latest",
			Labels: map[string]string{
				"io.docker.server.metadata": "name: My Image",
			},
			DigestString: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
		{
			Ref: "anotherimage:v1.0",
			Labels: map[string]string{
				"io.docker.server.metadata": "name: Another Image",
			},
			DigestString: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
	}))
}

// Test Catalog.Digest()
func TestCatalogDigest(t *testing.T) {
	catalog := Catalog{
		CatalogArtifact: CatalogArtifact{
			Title: "test-catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/test:latest",
					Tools: []string{"tool1", "tool2"},
				},
			},
		},
	}

	digest1, err := catalog.Digest()
	require.NoError(t, err)
	assert.NotEmpty(t, digest1)
	assert.Len(t, digest1, 64) // SHA256 hex string

	// Same catalog should produce same digest
	digest2, err := catalog.Digest()
	require.NoError(t, err)
	assert.Equal(t, digest1, digest2)
}

func TestCatalogDigestDifferentContent(t *testing.T) {
	catalog1 := Catalog{
		CatalogArtifact: CatalogArtifact{
			Title: "catalog1",
			Servers: []Server{
				{Type: workingset.ServerTypeImage, Image: "docker/test:v1"},
			},
		},
	}

	catalog2 := Catalog{
		CatalogArtifact: CatalogArtifact{
			Title: "catalog2",
			Servers: []Server{
				{Type: workingset.ServerTypeImage, Image: "docker/test:v2"},
			},
		},
	}

	digest1, err := catalog1.Digest()
	require.NoError(t, err)
	digest2, err := catalog2.Digest()
	require.NoError(t, err)
	assert.NotEqual(t, digest1, digest2)
}

func TestCatalogDigestWithTools(t *testing.T) {
	catalog1 := Catalog{
		CatalogArtifact: CatalogArtifact{
			Title: "test",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/test:latest",
					Tools: []string{"tool1", "tool2"},
				},
			},
		},
	}

	catalog2 := Catalog{
		CatalogArtifact: CatalogArtifact{
			Title: "test",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/test:latest",
					Tools: []string{"tool1"},
				},
			},
		},
	}

	digest1, err := catalog1.Digest()
	require.NoError(t, err)
	digest2, err := catalog2.Digest()
	require.NoError(t, err)
	assert.NotEqual(t, digest1, digest2)
}

// Test Catalog.Validate()
func TestCatalogValidateSuccess(t *testing.T) {
	tests := []struct {
		name    string
		catalog Catalog
	}{
		{
			name: "valid image server",
			catalog: Catalog{
				Ref: "test/catalog:latest",
				CatalogArtifact: CatalogArtifact{
					Title: "test",
					Servers: []Server{
						{
							Type:  workingset.ServerTypeImage,
							Image: "docker/test:latest",
						},
					},
				},
			},
		},
		{
			name: "valid registry server",
			catalog: Catalog{
				Ref: "test/catalog:latest",
				CatalogArtifact: CatalogArtifact{
					Title: "test",
					Servers: []Server{
						{
							Type:   workingset.ServerTypeRegistry,
							Source: "https://example.com/server",
						},
					},
				},
			},
		},
		{
			name: "multiple servers",
			catalog: Catalog{
				Ref: "test/catalog:latest",
				CatalogArtifact: CatalogArtifact{
					Title: "test",
					Servers: []Server{
						{Type: workingset.ServerTypeImage, Image: "docker/test:v1"},
						{Type: workingset.ServerTypeRegistry, Source: "https://example.com"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.catalog.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestCatalogValidateErrors(t *testing.T) {
	tests := []struct {
		name    string
		catalog Catalog
	}{
		{
			name: "empty title",
			catalog: Catalog{
				CatalogArtifact: CatalogArtifact{
					Title:   "",
					Servers: []Server{{Type: workingset.ServerTypeImage, Image: "test"}},
				},
			},
		},
		{
			name: "duplicate server name",
			catalog: Catalog{
				CatalogArtifact: CatalogArtifact{
					Title: "",
					Servers: []Server{
						{Type: workingset.ServerTypeImage, Image: "test", Snapshot: &workingset.ServerSnapshot{Server: catalog.Server{Name: "test"}}},
						{Type: workingset.ServerTypeImage, Image: "test", Snapshot: &workingset.ServerSnapshot{Server: catalog.Server{Name: "test"}}},
					},
				},
			},
		},
		{
			name: "invalid server type",
			catalog: Catalog{
				CatalogArtifact: CatalogArtifact{
					Title:   "test",
					Servers: []Server{{Type: "invalid"}},
				},
			},
		},
		{
			name: "image server without image",
			catalog: Catalog{
				CatalogArtifact: CatalogArtifact{
					Title:   "test",
					Servers: []Server{{Type: workingset.ServerTypeImage}},
				},
			},
		},
		{
			name: "registry server without source",
			catalog: Catalog{
				CatalogArtifact: CatalogArtifact{
					Title:   "test",
					Servers: []Server{{Type: workingset.ServerTypeRegistry}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.catalog.Validate()
			assert.Error(t, err)
		})
	}
}

// Test Catalog.ToDb() and NewFromDb()
func TestCatalogToDbAndFromDb(t *testing.T) {
	catalog := Catalog{
		Ref:    "test/catalog:latest",
		Source: "test-source",
		CatalogArtifact: CatalogArtifact{
			Title: "test-catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/test:latest",
					Tools: []string{"tool1", "tool2"},
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "test-server",
							Description: "Test",
						},
					},
				},
				{
					Type:   workingset.ServerTypeRegistry,
					Source: "https://example.com",
					Tools:  []string{"tool3"},
				},
			},
		},
	}

	dbCatalog, err := catalog.ToDb()
	require.NoError(t, err)

	// Verify conversion to DB format
	digest, err := catalog.Digest()
	require.NoError(t, err)
	assert.Equal(t, digest, dbCatalog.Digest)
	assert.Equal(t, catalog.Title, dbCatalog.Title)
	assert.Equal(t, catalog.Source, dbCatalog.Source)
	assert.Len(t, dbCatalog.Servers, 2)

	// Check first server (image)
	assert.Equal(t, string(workingset.ServerTypeImage), dbCatalog.Servers[0].ServerType)
	assert.Equal(t, "docker/test:latest", dbCatalog.Servers[0].Image)
	assert.Equal(t, []string{"tool1", "tool2"}, []string(dbCatalog.Servers[0].Tools))
	assert.NotNil(t, dbCatalog.Servers[0].Snapshot)
	assert.Equal(t, "test-server", dbCatalog.Servers[0].Snapshot.Server.Name)

	// Check second server (registry)
	assert.Equal(t, string(workingset.ServerTypeRegistry), dbCatalog.Servers[1].ServerType)
	assert.Equal(t, "https://example.com", dbCatalog.Servers[1].Source)
	assert.Equal(t, []string{"tool3"}, []string(dbCatalog.Servers[1].Tools))

	// Convert back from DB
	catalogWithDigest := NewFromDb(&dbCatalog)

	// Verify conversion from DB format
	assert.Equal(t, catalog.Title, catalogWithDigest.Title)
	assert.Equal(t, catalog.Source, catalogWithDigest.Source)
	assert.Equal(t, digest, catalogWithDigest.Digest)
	assert.Len(t, catalogWithDigest.Servers, 2)

	// Check first server roundtrip
	assert.Equal(t, catalog.Servers[0].Type, catalogWithDigest.Servers[0].Type)
	assert.Equal(t, catalog.Servers[0].Image, catalogWithDigest.Servers[0].Image)
	assert.Equal(t, catalog.Servers[0].Tools, catalogWithDigest.Servers[0].Tools)
	assert.NotNil(t, catalogWithDigest.Servers[0].Snapshot)

	// Check second server roundtrip
	assert.Equal(t, catalog.Servers[1].Type, catalogWithDigest.Servers[1].Type)
	assert.Equal(t, catalog.Servers[1].Source, catalogWithDigest.Servers[1].Source)
	assert.Equal(t, catalog.Servers[1].Tools, catalogWithDigest.Servers[1].Tools)
}
