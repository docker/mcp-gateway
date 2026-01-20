package catalognext

import (
	"path/filepath"
	"testing"
	"time"

	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
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

func getMockRegistryClient() registryapi.Client {
	server := v0.ServerResponse{
		Server: v0.ServerJSON{
			Version: "0.1.0",
			Packages: []model.Package{
				{
					RegistryType: "oci",
				},
			},
		},
		Meta: v0.ResponseMeta{
			Official: &v0.RegistryExtensions{
				IsLatest: true,
			},
		},
	}

	return mocks.NewMockRegistryAPIClient(mocks.WithServerListResponses(map[string]v0.ServerListResponse{
		"https://example.com/v0/servers/server1/versions": {
			Servers: []v0.ServerResponse{server},
		},
		"https://example.com/v0/servers/server2/versions": {
			Servers: []v0.ServerResponse{server},
		},
	}), mocks.WithServerResponses(map[string]v0.ServerResponse{
		"https://example.com/v0/servers/server1/versions/0.1.0": server,
		"https://example.com/v0/servers/server2/versions/0.1.0": server,
	}))
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

// Test NewPullOptionEvaluator
func TestNewPullOptionEvaluator(t *testing.T) {
	tests := []struct {
		name             string
		pullOptionParam  string
		pulledPreviously bool
		wantErr          bool
		wantOptions      []PullOptionConfig
	}{
		{
			name:             "empty string",
			pullOptionParam:  "",
			pulledPreviously: false,
			wantOptions:      []PullOptionConfig{},
		},
		{
			name:             "missing option",
			pullOptionParam:  "missing",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionMissing, Interval: 0},
			},
		},
		{
			name:             "never option",
			pullOptionParam:  "never",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionNever, Interval: 0},
			},
		},
		{
			name:             "always option",
			pullOptionParam:  "always",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionAlways, Interval: 0},
			},
		},
		{
			name:             "initial option",
			pullOptionParam:  "initial",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionInitial, Interval: 0},
			},
		},
		{
			name:             "exists option",
			pullOptionParam:  "exists",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionExists, Interval: 0},
			},
		},
		{
			name:             "duration only - hours",
			pullOptionParam:  "6h",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionDuration, Interval: 6 * time.Hour},
			},
		},
		{
			name:             "duration only - minutes",
			pullOptionParam:  "30m",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionDuration, Interval: 30 * time.Minute},
			},
		},
		{
			name:             "duration only - seconds",
			pullOptionParam:  "45s",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionDuration, Interval: 45 * time.Second},
			},
		},
		{
			name:             "always with duration",
			pullOptionParam:  "always@6h",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionAlways, Interval: 6 * time.Hour},
			},
		},
		{
			name:             "exists with duration",
			pullOptionParam:  "exists@1h30m",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionExists, Interval: 1*time.Hour + 30*time.Minute},
			},
		},
		{
			name:             "initial with duration",
			pullOptionParam:  "initial@24h",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionInitial, Interval: 24 * time.Hour},
			},
		},
		{
			name:             "combination initial+exists",
			pullOptionParam:  "initial+exists",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionInitial, Interval: 0},
				{PullOption: PullOptionExists, Interval: 0},
			},
		},
		{
			name:             "combination initial+exists with duration",
			pullOptionParam:  "initial+exists@6s",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionInitial, Interval: 0},
				{PullOption: PullOptionExists, Interval: 6 * time.Second},
			},
		},
		{
			name:             "combination missing+always with duration",
			pullOptionParam:  "missing+always@12h",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionMissing, Interval: 0},
				{PullOption: PullOptionAlways, Interval: 12 * time.Hour},
			},
		},
		{
			name:             "three options combined",
			pullOptionParam:  "initial+missing+exists@30m",
			pulledPreviously: false,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionInitial, Interval: 0},
				{PullOption: PullOptionMissing, Interval: 0},
				{PullOption: PullOptionExists, Interval: 30 * time.Minute},
			},
		},
		{
			name:             "pulledPreviously true with initial",
			pullOptionParam:  "initial",
			pulledPreviously: true,
			wantOptions: []PullOptionConfig{
				{PullOption: PullOptionInitial, Interval: 0},
			},
		},
		{
			name:             "invalid pull option",
			pullOptionParam:  "invalid",
			pulledPreviously: false,
			wantErr:          true,
		},
		{
			name:             "invalid duration format",
			pullOptionParam:  "always@invalid",
			pulledPreviously: false,
			wantErr:          true,
		},
		{
			name:             "invalid option with duration",
			pullOptionParam:  "invalid@6h",
			pulledPreviously: false,
			wantErr:          true,
		},
		{
			name:             "negative duration",
			pullOptionParam:  "-1h",
			pulledPreviously: false,
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator, err := NewPullOptionEvaluator(tt.pullOptionParam, tt.pulledPreviously)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, evaluator)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, evaluator)
			assert.Equal(t, tt.pulledPreviously, evaluator.pulledPreviously)
			assert.Len(t, evaluator.pullOptions, len(tt.wantOptions))

			for i, wantOption := range tt.wantOptions {
				assert.Equal(t, wantOption.PullOption, evaluator.pullOptions[i].PullOption, "PullOption mismatch at index %d", i)
				assert.Equal(t, wantOption.Interval, evaluator.pullOptions[i].Interval, "Interval mismatch at index %d", i)
			}
		})
	}
}

func TestPullOptionEvaluatorIsAlways(t *testing.T) {
	tests := []struct {
		name            string
		pullOptionParam string
		wantIsAlways    bool
	}{
		{
			name:            "always without interval",
			pullOptionParam: "always",
			wantIsAlways:    true,
		},
		{
			name:            "always with interval",
			pullOptionParam: "always@6h",
			wantIsAlways:    false,
		},
		{
			name:            "missing option",
			pullOptionParam: "missing",
			wantIsAlways:    false,
		},
		{
			name:            "never option",
			pullOptionParam: "never",
			wantIsAlways:    false,
		},
		{
			name:            "initial option",
			pullOptionParam: "initial",
			wantIsAlways:    false,
		},
		{
			name:            "exists option",
			pullOptionParam: "exists",
			wantIsAlways:    false,
		},
		{
			name:            "duration only",
			pullOptionParam: "6h",
			wantIsAlways:    false,
		},
		{
			name:            "combination with always no interval",
			pullOptionParam: "initial+always",
			wantIsAlways:    true,
		},
		{
			name:            "combination with always with interval",
			pullOptionParam: "initial+always@6h",
			wantIsAlways:    false,
		},
		{
			name:            "empty string",
			pullOptionParam: "",
			wantIsAlways:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator, err := NewPullOptionEvaluator(tt.pullOptionParam, false)
			require.NoError(t, err)
			assert.Equal(t, tt.wantIsAlways, evaluator.IsAlways())
		})
	}
}

func TestPullOptionEvaluatorEvaluate(t *testing.T) {
	now := time.Now()
	ago := func(dur time.Duration) *time.Time {
		tm := now.Add(-dur)
		return &tm
	}

	tests := []struct {
		name             string
		pullOptionParam  string
		pulledPreviously bool
		dbCatalog        *db.Catalog
		shouldPull       bool
	}{
		// Empty option tests
		{
			name:             "empty option with nil catalog",
			pullOptionParam:  "",
			pulledPreviously: false,
			dbCatalog:        nil,
			shouldPull:       false,
		},
		{
			name:             "empty option with existing catalog",
			pullOptionParam:  "",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(time.Hour)},
			shouldPull:       false,
		},

		// Missing option tests
		{
			name:             "missing with nil catalog",
			pullOptionParam:  "missing",
			pulledPreviously: false,
			dbCatalog:        nil,
			shouldPull:       true,
		},
		{
			name:             "missing with existing catalog",
			pullOptionParam:  "missing",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(time.Hour)},
			shouldPull:       false,
		},

		// Never option tests
		{
			name:             "never with nil catalog",
			pullOptionParam:  "never",
			pulledPreviously: false,
			dbCatalog:        nil,
			shouldPull:       false,
		},
		{
			name:             "never with existing catalog",
			pullOptionParam:  "never",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(time.Hour)},
			shouldPull:       false,
		},

		// Always option tests
		{
			name:             "always with nil catalog",
			pullOptionParam:  "always",
			pulledPreviously: false,
			dbCatalog:        nil,
			shouldPull:       true,
		},
		{
			name:             "always with existing catalog",
			pullOptionParam:  "always",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(time.Hour)},
			shouldPull:       true,
		},

		// Initial option tests
		{
			name:             "initial with nil catalog not pulled previously",
			pullOptionParam:  "initial",
			pulledPreviously: false,
			dbCatalog:        nil,
			shouldPull:       true,
		},
		{
			name:             "initial with nil catalog pulled previously",
			pullOptionParam:  "initial",
			pulledPreviously: true,
			dbCatalog:        nil,
			shouldPull:       false,
		},
		{
			name:             "initial with existing catalog",
			pullOptionParam:  "initial",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(time.Hour)},
			shouldPull:       false,
		},

		// Exists option tests
		{
			name:             "exists with nil catalog",
			pullOptionParam:  "exists",
			pulledPreviously: false,
			dbCatalog:        nil,
			shouldPull:       false,
		},
		{
			name:             "exists with existing catalog no interval",
			pullOptionParam:  "exists",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(time.Hour)},
			shouldPull:       true,
		},

		// Duration only tests
		{
			name:             "duration only with nil catalog",
			pullOptionParam:  "6h",
			pulledPreviously: false,
			dbCatalog:        nil,
			shouldPull:       true,
		},
		{
			name:             "duration only with catalog updated recently",
			pullOptionParam:  "6h",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(time.Hour)},
			shouldPull:       false,
		},
		{
			name:             "duration only with catalog updated long ago",
			pullOptionParam:  "30m",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(2 * time.Hour)},
			shouldPull:       true,
		},

		// Always with duration tests
		{
			name:             "always@1h with catalog updated 30m ago",
			pullOptionParam:  "always@1h",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(30 * time.Minute)},
			shouldPull:       false,
		},
		{
			name:             "always@1h with catalog updated 2h ago",
			pullOptionParam:  "always@1h",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(2 * time.Hour)},
			shouldPull:       true,
		},

		// Exists with duration tests
		{
			name:             "exists@1h with nil catalog",
			pullOptionParam:  "exists@1h",
			pulledPreviously: false,
			dbCatalog:        nil,
			shouldPull:       false,
		},
		{
			name:             "exists@1h with catalog updated 30m ago",
			pullOptionParam:  "exists@1h",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(30 * time.Minute)},
			shouldPull:       false,
		},
		{
			name:             "exists@1h with catalog updated 2h ago",
			pullOptionParam:  "exists@1h",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(2 * time.Hour)},
			shouldPull:       true,
		},

		// Combination tests: initial+exists
		{
			name:             "initial+exists with nil catalog not pulled previously",
			pullOptionParam:  "initial+exists",
			pulledPreviously: false,
			dbCatalog:        nil,
			shouldPull:       true, // initial triggers
		},
		{
			name:             "initial+exists with nil catalog pulled previously",
			pullOptionParam:  "initial+exists",
			pulledPreviously: true,
			dbCatalog:        nil,
			shouldPull:       false, // initial doesn't trigger, exists doesn't trigger for nil
		},
		{
			name:             "initial+exists with existing catalog",
			pullOptionParam:  "initial+exists",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(time.Hour)},
			shouldPull:       true, // exists triggers (no interval means always)
		},

		// Combination tests: initial+exists@6s
		{
			name:             "initial+exists@6s with nil catalog not pulled previously",
			pullOptionParam:  "initial+exists@6s",
			pulledPreviously: false,
			dbCatalog:        nil,
			shouldPull:       true, // initial triggers
		},
		{
			name:             "initial+exists@6s with nil catalog pulled previously",
			pullOptionParam:  "initial+exists@6s",
			pulledPreviously: true,
			dbCatalog:        nil,
			shouldPull:       false, // neither triggers
		},
		{
			name:             "initial+exists@30s with catalog updated 15s ago",
			pullOptionParam:  "initial+exists@30s",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(15 * time.Second)},
			shouldPull:       false, // exists@30s doesn't trigger (only 15s elapsed)
		},
		{
			name:             "initial+exists@6s with catalog updated 30s ago",
			pullOptionParam:  "initial+exists@6s",
			pulledPreviously: false,
			dbCatalog:        &db.Catalog{Ref: "test", LastUpdated: ago(30 * time.Second)},
			shouldPull:       true, // exists@6s triggers (30s > 6s)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator, err := NewPullOptionEvaluator(tt.pullOptionParam, tt.pulledPreviously)
			require.NoError(t, err)

			result := evaluator.Evaluate(tt.dbCatalog)
			assert.Equal(t, tt.shouldPull, result)
		})
	}
}
