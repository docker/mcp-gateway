package workingset

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/test/mocks"
	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
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

func TestNewFromDb(t *testing.T) {
	dbSet := &db.WorkingSet{
		ID:   "test-id",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:   "registry",
				Source: "https://example.com/server",
				Config: map[string]any{"key": "value"},
				Tools:  []string{"tool1", "tool2"},
			},
			{
				Type:  "image",
				Image: "docker/test:latest",
			},
		},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	}

	workingSet := NewFromDb(dbSet)

	assert.Equal(t, "test-id", workingSet.ID)
	assert.Equal(t, "Test Working Set", workingSet.Name)
	assert.Equal(t, CurrentWorkingSetVersion, workingSet.Version)
	assert.Len(t, workingSet.Servers, 2)

	// Check registry server
	assert.Equal(t, ServerTypeRegistry, workingSet.Servers[0].Type)
	assert.Equal(t, "https://example.com/server", workingSet.Servers[0].Source)
	assert.Equal(t, map[string]any{"key": "value"}, workingSet.Servers[0].Config)
	assert.Equal(t, []string{"tool1", "tool2"}, workingSet.Servers[0].Tools)

	// Check image server
	assert.Equal(t, ServerTypeImage, workingSet.Servers[1].Type)
	assert.Equal(t, "docker/test:latest", workingSet.Servers[1].Image)

	// Check secrets
	assert.Len(t, workingSet.Secrets, 1)
	assert.Equal(t, SecretProviderDockerDesktop, workingSet.Secrets["default"].Provider)
}

func TestWorkingSetToDb(t *testing.T) {
	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-id",
		Name:    "Test Working Set",
		Servers: []Server{
			{
				Type:   ServerTypeRegistry,
				Source: "https://example.com/server",
				Config: map[string]any{"key": "value"},
				Tools:  []string{"tool1", "tool2"},
			},
			{
				Type:  ServerTypeImage,
				Image: "docker/test:latest",
			},
		},
		Secrets: map[string]Secret{
			"default": {Provider: SecretProviderDockerDesktop},
		},
	}

	dbSet := workingSet.ToDb()

	assert.Equal(t, "test-id", dbSet.ID)
	assert.Equal(t, "Test Working Set", dbSet.Name)
	assert.Len(t, dbSet.Servers, 2)

	// Check registry server
	assert.Equal(t, "registry", dbSet.Servers[0].Type)
	assert.Equal(t, "https://example.com/server", dbSet.Servers[0].Source)
	assert.Equal(t, map[string]any{"key": "value"}, dbSet.Servers[0].Config)
	assert.Equal(t, []string{"tool1", "tool2"}, dbSet.Servers[0].Tools)

	// Check image server
	assert.Equal(t, "image", dbSet.Servers[1].Type)
	assert.Equal(t, "docker/test:latest", dbSet.Servers[1].Image)

	// Check secrets
	assert.Len(t, dbSet.Secrets, 1)
	assert.Equal(t, "docker-desktop-store", dbSet.Secrets["default"].Provider)
}

func TestWorkingSetRoundTrip(t *testing.T) {
	original := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-id",
		Name:    "Test Working Set",
		Servers: []Server{
			{
				Type:    ServerTypeRegistry,
				Source:  "https://example.com/server",
				Config:  map[string]any{"key": "value"},
				Secrets: "default",
				Tools:   []string{"tool1", "tool2"},
			},
		},
		Secrets: map[string]Secret{
			"default": {Provider: SecretProviderDockerDesktop},
		},
	}

	// Convert to DB and back
	dbSet := original.ToDb()
	roundTripped := NewFromDb(&dbSet)

	assert.Equal(t, original.ID, roundTripped.ID)
	assert.Equal(t, original.Name, roundTripped.Name)
	assert.Equal(t, original.Version, roundTripped.Version)
	assert.Equal(t, original.Servers, roundTripped.Servers)
	assert.Equal(t, original.Secrets, roundTripped.Secrets)
}

func TestWorkingSetValidate(t *testing.T) {
	tests := []struct {
		name      string
		ws        WorkingSet
		expectErr bool
	}{
		{
			name: "valid registry server",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				ID:      "test-id",
				Name:    "Test",
				Servers: []Server{
					{
						Type:   ServerTypeRegistry,
						Source: "https://example.com/server",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "valid image server",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				ID:      "test-id",
				Name:    "Test",
				Servers: []Server{
					{
						Type:  ServerTypeImage,
						Image: "docker/test:latest",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "missing version",
			ws: WorkingSet{
				ID:      "test-id",
				Name:    "Test",
				Servers: []Server{},
			},
			expectErr: true,
		},
		{
			name: "missing ID",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				Name:    "Test",
				Servers: []Server{},
			},
			expectErr: true,
		},
		{
			name: "missing name",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				ID:      "test-id",
				Servers: []Server{},
			},
			expectErr: true,
		},
		{
			name: "registry server missing source",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				ID:      "test-id",
				Name:    "Test",
				Servers: []Server{
					{
						Type: ServerTypeRegistry,
					},
				},
			},
			expectErr: true,
		},
		{
			name: "image server missing image",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				ID:      "test-id",
				Name:    "Test",
				Servers: []Server{
					{
						Type: ServerTypeImage,
					},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid server type",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				ID:      "test-id",
				Name:    "Test",
				Servers: []Server{
					{
						Type: ServerType("invalid"),
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ws.Validate()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateWorkingSetID(t *testing.T) {
	tests := []struct {
		name        string
		inputName   string
		existingIDs []string
		expectedID  string
	}{
		{
			name:       "simple name",
			inputName:  "MyWorkingSet",
			expectedID: "myworkingset",
		},
		{
			name:       "name with spaces",
			inputName:  "My Working Set",
			expectedID: "my-working-set",
		},
		{
			name:       "name with special characters",
			inputName:  "My@Working#Set!",
			expectedID: "my-working-set-",
		},
		{
			name:        "name with collision",
			inputName:   "test",
			existingIDs: []string{"test"},
			expectedID:  "test-2",
		},
		{
			name:        "name with multiple collisions",
			inputName:   "test",
			existingIDs: []string{"test", "test-2", "test-3"},
			expectedID:  "test-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh database for each subtest to avoid ID conflicts
			dao := setupTestDB(t)
			ctx := t.Context()

			// Setup: create existing working sets
			for _, id := range tt.existingIDs {
				err := dao.CreateWorkingSet(ctx, db.WorkingSet{
					ID:      id,
					Name:    "Existing",
					Servers: db.ServerList{},
					Secrets: db.SecretMap{},
				})
				require.NoError(t, err)
			}

			id, err := createWorkingSetID(ctx, tt.inputName, dao)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedID, id)
		})
	}
}

func TestResolveServerFromString(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expected        Server
		expectedVersion string
		expectError     bool
	}{
		{
			name:  "docker image",
			input: "docker://myimage:latest",
			expected: Server{
				Type:  ServerTypeImage,
				Image: "myimage:latest@sha256:1234567890",
				Snapshot: &ServerSnapshot{
					Server: catalog.Server{
						Name: "My Image",
					},
				},
				Secrets: "default",
			},
			expectedVersion: "latest",
		},
		{
			name:  "http registry",
			input: "http://example.com/v0/servers/my-server",
			expected: Server{
				Type:    ServerTypeRegistry,
				Source:  "http://example.com/v0/servers/my-server/versions/latest",
				Secrets: "default",
			},
			expectedVersion: "latest",
		},
		{
			name:  "https registry",
			input: "https://example.com/v0/servers/my-server",
			expected: Server{
				Type:    ServerTypeRegistry,
				Source:  "https://example.com/v0/servers/my-server/versions/latest",
				Secrets: "default",
			},
			expectedVersion: "latest",
		},
		{
			name:  "specific version registry",
			input: "https://example.com/v0/servers/my-server/versions/0.1.0",
			expected: Server{
				Type:    ServerTypeRegistry,
				Source:  "https://example.com/v0/servers/my-server/versions/0.1.0",
				Secrets: "default",
			},
			expectedVersion: "0.1.0",
		},
		{
			name:        "invalid format",
			input:       "invalid-format",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverResponse := v0.ServerResponse{
				Server: v0.ServerJSON{
					Version: tt.expectedVersion,
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
			registryClient := mocks.NewMockRegistryAPIClient(mocks.WithServerListResponses(map[string]v0.ServerListResponse{
				"http://example.com/v0/servers/my-server/versions": {
					Servers: []v0.ServerResponse{serverResponse},
				},
				"https://example.com/v0/servers/my-server/versions": {
					Servers: []v0.ServerResponse{serverResponse},
				},
			}), mocks.WithServerResponses(map[string]v0.ServerResponse{
				"http://example.com/v0/servers/my-server/versions/" + tt.expectedVersion: serverResponse,
			}))

			ociService := mocks.NewMockOCIService(mocks.WithDigests(map[string]string{
				"myimage:latest": "sha256:1234567890",
			}), mocks.WithLabels(map[string]map[string]string{
				"myimage:latest": {
					"io.docker.server.metadata": "name: My Image",
				},
			}))

			server, err := resolveServerFromString(t.Context(), registryClient, ociService, tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, server)
			}
		})
	}
}

func TestResolveServerFromStringResolvesLatestVersion(t *testing.T) {
	serverResponse := v0.ServerResponse{
		Server: v0.ServerJSON{
			Version: "0.2.0",
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
	oldServerResponse := v0.ServerResponse{
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
				IsLatest: false,
			},
		},
	}
	registryClient := mocks.NewMockRegistryAPIClient(mocks.WithServerListResponses(map[string]v0.ServerListResponse{
		"http://example.com/v0/servers/my-server/versions": {
			Servers: []v0.ServerResponse{serverResponse, oldServerResponse},
		},
	}), mocks.WithServerResponses(map[string]v0.ServerResponse{
		"http://example.com/v0/servers/my-server/versions/0.1.0": oldServerResponse,
		"http://example.com/v0/servers/my-server/versions/0.2.0": serverResponse,
	}))

	server, err := resolveServerFromString(t.Context(), registryClient, mocks.NewMockOCIService(), "http://example.com/v0/servers/my-server")
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/v0/servers/my-server/versions/0.2.0", server.Source)
}

func TestResolveSnapshot(t *testing.T) {
	tests := []struct {
		name        string
		server      Server
		labels      map[string]string
		expectError bool
		expected    *ServerSnapshot
	}{
		{
			name: "valid image with metadata",
			server: Server{
				Type:  ServerTypeImage,
				Image: "testimage:v1.0",
			},
			labels: map[string]string{
				"io.docker.server.metadata": `name: Test Server
type: remote
image: testimage:v1.0
description: A test server for unit tests
title: Test Server Title`,
			},
			expectError: false,
			expected: &ServerSnapshot{
				Server: catalog.Server{
					Name:        "Test Server",
					Type:        "remote",
					Image:       "testimage:v1.0",
					Description: "A test server for unit tests",
					Title:       "Test Server Title",
				},
			},
		},
		{
			name: "image with minimal metadata",
			server: Server{
				Type:  ServerTypeImage,
				Image: "minimalimage:latest",
			},
			labels: map[string]string{
				"io.docker.server.metadata": `name: Minimal
type: remote`,
			},
			expectError: false,
			expected: &ServerSnapshot{
				Server: catalog.Server{
					Name: "Minimal",
					Type: "remote",
				},
			},
		},
		{
			name: "invalid image reference",
			server: Server{
				Type:  ServerTypeImage,
				Image: "invalid::reference",
			},
			labels:      map[string]string{},
			expectError: true,
		},
		{
			name: "missing metadata label",
			server: Server{
				Type:  ServerTypeImage,
				Image: "nometadata:latest",
			},
			labels:      map[string]string{},
			expectError: true,
		},
		{
			name: "invalid yaml in metadata",
			server: Server{
				Type:  ServerTypeImage,
				Image: "badyaml:latest",
			},
			labels: map[string]string{
				"io.docker.server.metadata": "invalid: yaml: [syntax",
			},
			expectError: true,
		},
		{
			name: "image with digest",
			server: Server{
				Type:  ServerTypeImage,
				Image: "registry.example.com/myimage@sha256:abcdef123456abcdef123456abcdef123456abcdef123456abcdef123456abcd",
			},
			labels: map[string]string{
				"io.docker.server.metadata": `name: Digested Image
type: remote`,
			},
			expectError: false,
			expected: &ServerSnapshot{
				Server: catalog.Server{
					Name: "Digested Image",
					Type: "remote",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociService := mocks.NewMockOCIService(mocks.WithLabels(map[string]map[string]string{
				tt.server.Image: tt.labels,
			}))
			ctx := t.Context()

			snapshot, err := resolveSnapshot(ctx, ociService, tt.server)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, snapshot)
				assert.Equal(t, tt.expected.Server.Name, snapshot.Server.Name)
				assert.Equal(t, tt.expected.Server.Type, snapshot.Server.Type)
				if tt.expected.Server.Description != "" {
					assert.Equal(t, tt.expected.Server.Description, snapshot.Server.Description)
				}
			}
		})
	}
}
