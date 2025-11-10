package catalognext

import (
	"path/filepath"
	"testing"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// Test Catalog.Digest()
func TestCatalogDigest(t *testing.T) {
	catalog := Catalog{
		Name: "test-catalog",
		Servers: []Server{
			{
				Type:  workingset.ServerTypeImage,
				Image: "docker/test:latest",
				Tools: []string{"tool1", "tool2"},
			},
		},
	}

	digest1 := catalog.Digest()
	assert.NotEmpty(t, digest1)
	assert.Len(t, digest1, 64) // SHA256 hex string

	// Same catalog should produce same digest
	digest2 := catalog.Digest()
	assert.Equal(t, digest1, digest2)
}

func TestCatalogDigestDifferentContent(t *testing.T) {
	catalog1 := Catalog{
		Name: "catalog1",
		Servers: []Server{
			{Type: workingset.ServerTypeImage, Image: "docker/test:v1"},
		},
	}

	catalog2 := Catalog{
		Name: "catalog2",
		Servers: []Server{
			{Type: workingset.ServerTypeImage, Image: "docker/test:v2"},
		},
	}

	assert.NotEqual(t, catalog1.Digest(), catalog2.Digest())
}

func TestCatalogDigestExcludesSource(t *testing.T) {
	// Source should not affect digest since it's metadata
	catalog1 := Catalog{
		Name:   "test",
		Source: "source1",
		Servers: []Server{
			{Type: workingset.ServerTypeImage, Image: "docker/test:latest"},
		},
	}

	catalog2 := Catalog{
		Name:   "test",
		Source: "source2",
		Servers: []Server{
			{Type: workingset.ServerTypeImage, Image: "docker/test:latest"},
		},
	}

	assert.Equal(t, catalog1.Digest(), catalog2.Digest())
}

func TestCatalogDigestWithTools(t *testing.T) {
	catalog1 := Catalog{
		Name: "test",
		Servers: []Server{
			{
				Type:  workingset.ServerTypeImage,
				Image: "docker/test:latest",
				Tools: []string{"tool1", "tool2"},
			},
		},
	}

	catalog2 := Catalog{
		Name: "test",
		Servers: []Server{
			{
				Type:  workingset.ServerTypeImage,
				Image: "docker/test:latest",
				Tools: []string{"tool1"},
			},
		},
	}

	assert.NotEqual(t, catalog1.Digest(), catalog2.Digest())
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
				Name: "test",
				Servers: []Server{
					{
						Type:  workingset.ServerTypeImage,
						Image: "docker/test:latest",
					},
				},
			},
		},
		{
			name: "valid registry server",
			catalog: Catalog{
				Name: "test",
				Servers: []Server{
					{
						Type:   workingset.ServerTypeRegistry,
						Source: "https://example.com/server",
					},
				},
			},
		},
		{
			name: "multiple servers",
			catalog: Catalog{
				Name: "test",
				Servers: []Server{
					{Type: workingset.ServerTypeImage, Image: "docker/test:v1"},
					{Type: workingset.ServerTypeRegistry, Source: "https://example.com"},
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
			name: "empty name",
			catalog: Catalog{
				Name:    "",
				Servers: []Server{{Type: workingset.ServerTypeImage, Image: "test"}},
			},
		},
		{
			name: "invalid server type",
			catalog: Catalog{
				Name:    "test",
				Servers: []Server{{Type: "invalid"}},
			},
		},
		{
			name: "image server without image",
			catalog: Catalog{
				Name:    "test",
				Servers: []Server{{Type: workingset.ServerTypeImage}},
			},
		},
		{
			name: "registry server without source",
			catalog: Catalog{
				Name:    "test",
				Servers: []Server{{Type: workingset.ServerTypeRegistry}},
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
		Name:   "test-catalog",
		Source: "test-source",
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
	}

	dbCatalog := catalog.ToDb()

	// Verify conversion to DB format
	assert.Equal(t, catalog.Digest(), dbCatalog.Digest)
	assert.Equal(t, catalog.Name, dbCatalog.Name)
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
	assert.Equal(t, catalog.Name, catalogWithDigest.Name)
	assert.Equal(t, catalog.Source, catalogWithDigest.Source)
	assert.Equal(t, catalog.Digest(), catalogWithDigest.Digest)
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
