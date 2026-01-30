package workingset

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/test/mocks"
)

//go:embed testdata/*
var testData embed.FS

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
	assert.Equal(t, ToolList([]string{"tool1", "tool2"}), workingSet.Servers[0].Tools)

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

func TestNewFromDbWithRemoteServer(t *testing.T) {
	dbSet := &db.WorkingSet{
		ID:   "test-remote-id",
		Name: "Test Remote Working Set",
		Servers: db.ServerList{
			{
				Type:     "remote",
				Endpoint: "https://mcp.example.com/sse",
				Tools:    []string{"tool1", "tool2"},
			},
		},
		Secrets: db.SecretMap{},
	}

	workingSet := NewFromDb(dbSet)

	assert.Equal(t, "test-remote-id", workingSet.ID)
	assert.Equal(t, "Test Remote Working Set", workingSet.Name)
	assert.Equal(t, CurrentWorkingSetVersion, workingSet.Version)
	assert.Len(t, workingSet.Servers, 1)

	// Check remote server
	assert.Equal(t, ServerTypeRemote, workingSet.Servers[0].Type)
	assert.Equal(t, "https://mcp.example.com/sse", workingSet.Servers[0].Endpoint)
	assert.Equal(t, ToolList([]string{"tool1", "tool2"}), workingSet.Servers[0].Tools)
}

func TestWorkingSetToDbWithRemoteServer(t *testing.T) {
	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-remote-id",
		Name:    "Test Remote Working Set",
		Servers: []Server{
			{
				Type:     ServerTypeRemote,
				Endpoint: "https://mcp.example.com/sse",
				Tools:    []string{"tool1", "tool2"},
			},
		},
		Secrets: map[string]Secret{},
	}

	dbSet := workingSet.ToDb()

	assert.Equal(t, "test-remote-id", dbSet.ID)
	assert.Equal(t, "Test Remote Working Set", dbSet.Name)
	assert.Len(t, dbSet.Servers, 1)

	// Check remote server
	assert.Equal(t, "remote", dbSet.Servers[0].Type)
	assert.Equal(t, "https://mcp.example.com/sse", dbSet.Servers[0].Endpoint)
	assert.Equal(t, []string{"tool1", "tool2"}, dbSet.Servers[0].Tools)
}

func TestWorkingSetRoundTripWithRemoteServer(t *testing.T) {
	original := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-remote-id",
		Name:    "Test Remote Working Set",
		Servers: []Server{
			{
				Type:     ServerTypeRemote,
				Endpoint: "https://mcp.example.com/sse",
				Tools:    []string{"tool1", "tool2"},
				Config:   map[string]any{"timeout": 30},
			},
		},
		Secrets: map[string]Secret{},
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

func TestWorkingSetWithMixedServerTypes(t *testing.T) {
	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-mixed-id",
		Name:    "Test Mixed Working Set",
		Servers: []Server{
			{
				Type:   ServerTypeRegistry,
				Source: "https://registry.example.com",
				Tools:  []string{"registry-tool"},
			},
			{
				Type:  ServerTypeImage,
				Image: "docker/test:latest",
				Tools: []string{"image-tool"},
			},
			{
				Type:     ServerTypeRemote,
				Endpoint: "https://mcp.example.com/sse",
				Tools:    []string{"remote-tool"},
			},
		},
		Secrets: map[string]Secret{},
	}

	// Convert to DB and back
	dbSet := workingSet.ToDb()
	roundTripped := NewFromDb(&dbSet)

	assert.Equal(t, workingSet.ID, roundTripped.ID)
	assert.Equal(t, workingSet.Name, roundTripped.Name)
	assert.Len(t, roundTripped.Servers, 3)

	// Verify registry server
	assert.Equal(t, ServerTypeRegistry, roundTripped.Servers[0].Type)
	assert.Equal(t, "https://registry.example.com", roundTripped.Servers[0].Source)
	assert.Empty(t, roundTripped.Servers[0].Image)
	assert.Empty(t, roundTripped.Servers[0].Endpoint)

	// Verify image server
	assert.Equal(t, ServerTypeImage, roundTripped.Servers[1].Type)
	assert.Equal(t, "docker/test:latest", roundTripped.Servers[1].Image)
	assert.Empty(t, roundTripped.Servers[1].Source)
	assert.Empty(t, roundTripped.Servers[1].Endpoint)

	// Verify remote server
	assert.Equal(t, ServerTypeRemote, roundTripped.Servers[2].Type)
	assert.Equal(t, "https://mcp.example.com/sse", roundTripped.Servers[2].Endpoint)
	assert.Empty(t, roundTripped.Servers[2].Source)
	assert.Empty(t, roundTripped.Servers[2].Image)
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
			name: "valid remote server",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				ID:      "test-id",
				Name:    "Test",
				Servers: []Server{
					{
						Type:     ServerTypeRemote,
						Endpoint: "https://mcp.example.com/sse",
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
			name: "remote server missing endpoint",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				ID:      "test-id",
				Name:    "Test",
				Servers: []Server{
					{
						Type: ServerTypeRemote,
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
		{
			// More details tests of the server snapshot are in other tests below.
			name: "invalid server snapshot",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				ID:      "test-id",
				Name:    "Test",
				Servers: []Server{
					{
						Type: ServerTypeImage,
						Snapshot: &ServerSnapshot{
							Server: catalog.Server{
								// Fails due to missing info like config
								Name: "test-server",
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "duplicate server name",
			ws: WorkingSet{
				Version: CurrentWorkingSetVersion,
				ID:      "test-id",
				Name:    "Test",
				Servers: []Server{
					{
						Type:  ServerTypeImage,
						Image: "myimage:latest",
						Snapshot: &ServerSnapshot{
							Server: catalog.Server{
								Name: "mcp.docker.com/test-server",
							},
						},
					},
					{
						Type:  ServerTypeImage,
						Image: "myimage:previous",
						Snapshot: &ServerSnapshot{
							Server: catalog.Server{
								Name: "mcp.docker.com/test-server",
							},
						},
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

func TestValidateServerSnapshot(t *testing.T) {
	tests := []struct {
		name      string
		snapshot  ServerSnapshot
		expectErr string
	}{
		{
			name: "valid server snapshot",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "test-server",
							"description": "test-server",
							"type":        "object",
							"properties": map[string]any{
								"mode": map[string]any{
									"type": "string",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "valid with multiple property types",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "config description",
							"type":        "object",
							"properties": map[string]any{
								"strProp": map[string]any{
									"type": "string",
								},
								"intProp": map[string]any{
									"type": "integer",
								},
								"numProp": map[string]any{
									"type": "number",
								},
								"boolProp": map[string]any{
									"type": "boolean",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "valid with nested object",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "config description",
							"type":        "object",
							"properties": map[string]any{
								"nested": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"innerProp": map[string]any{
											"type": "string",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "valid with array of strings",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "config description",
							"type":        "object",
							"properties": map[string]any{
								"tags": map[string]any{
									"type": "array",
									"items": map[string]any{
										"type": "string",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "valid with nil config",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name:   "test-server",
					Config: nil,
				},
			},
		},
		{
			name:      "config item is not a map",
			expectErr: "config[0] is not a map",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name:   "test-server",
					Config: []any{"not a map"},
				},
			},
		},
		{
			name:      "config item missing name field",
			expectErr: "config[0] has no name field",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"description": "desc",
							"type":        "object",
						},
					},
				},
			},
		},
		{
			name:      "config item missing description field",
			expectErr: "config[0] has no description field",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name": "config",
							"type": "object",
						},
					},
				},
			},
		},
		{
			name:      "config item missing type field",
			expectErr: "config[0] has no type field",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
						},
					},
				},
			},
		},
		{
			name:      "config item type is not object",
			expectErr: "config[0].type must be 'object', got 'string'",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
							"type":        "string",
						},
					},
				},
			},
		},
		{
			name:      "config item properties is not a map",
			expectErr: "config[0].properties is not a map",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
							"type":        "object",
							"properties":  "not a map",
						},
					},
				},
			},
		},
		{
			name:      "property is not a map",
			expectErr: "config[0].properties.badProp is not a map",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
							"type":        "object",
							"properties": map[string]any{
								"badProp": "not a map",
							},
						},
					},
				},
			},
		},
		{
			name:      "property missing type field",
			expectErr: "config[0].properties.noType has no type field",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
							"type":        "object",
							"properties": map[string]any{
								"noType": map[string]any{
									"description": "missing type",
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "property has unsupported type",
			expectErr: "config[0].properties.badType.type null is not supported",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
							"type":        "object",
							"properties": map[string]any{
								"badType": map[string]any{
									"type": "null",
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "nested object missing properties",
			expectErr: "config[0].properties.nested is type 'object' but has no properties field",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
							"type":        "object",
							"properties": map[string]any{
								"nested": map[string]any{
									"type": "object",
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "array missing items field",
			expectErr: "config[0].properties.arr is type 'array' but has no items field",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
							"type":        "object",
							"properties": map[string]any{
								"arr": map[string]any{
									"type": "array",
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "array items missing type",
			expectErr: "config[0].properties.arr.items has no type field",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
							"type":        "object",
							"properties": map[string]any{
								"arr": map[string]any{
									"type":  "array",
									"items": map[string]any{},
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "array items type must be string",
			expectErr: "config[0].properties.arr.items type must be string",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
							"type":        "object",
							"properties": map[string]any{
								"arr": map[string]any{
									"type": "array",
									"items": map[string]any{
										"type": "integer",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "error in second config item",
			expectErr: "config[1] has no name field",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config1",
							"description": "desc",
							"type":        "object",
							"properties": map[string]any{
								"prop": map[string]any{
									"type": "string",
								},
							},
						},
						map[string]any{
							"description": "missing name",
							"type":        "object",
						},
					},
				},
			},
		},
		{
			name:      "deeply nested object validation error",
			expectErr: "config[0].properties.level1.level2 has no type field",
			snapshot: ServerSnapshot{
				Server: catalog.Server{
					Name: "test-server",
					Config: []any{
						map[string]any{
							"name":        "config",
							"description": "desc",
							"type":        "object",
							"properties": map[string]any{
								"level1": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"level2": map[string]any{
											"description": "missing type",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.snapshot.ValidateInnerConfig()
			if tt.expectErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expectErr)
			} else {
				require.NoError(t, err)
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
		expected        []Server
		expectedVersion string
		expectError     bool
	}{
		{
			name:  "local docker image",
			input: "docker://myimage:latest",
			expected: []Server{{
				Type:  ServerTypeImage,
				Image: "myimage:latest",
				Snapshot: &ServerSnapshot{
					Server: catalog.Server{
						Name:  "My Image",
						Type:  "server",
						Image: "myimage:latest",
					},
				},
				Secrets: "default",
			}},
			expectedVersion: "latest",
		},
		{
			name:  "remote docker image",
			input: "docker://bobbarker/myimage:latest",
			expected: []Server{{
				Type:  ServerTypeImage,
				Image: "bobbarker/myimage:latest@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				Snapshot: &ServerSnapshot{
					Server: catalog.Server{
						Name:  "My Image",
						Type:  "server",
						Image: "bobbarker/myimage:latest@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					},
				},
				Secrets: "default",
			}},
			expectedVersion: "latest",
		},
		{
			name:  "http registry",
			input: "http://example.com/v0/servers/my-server",
			expected: []Server{{
				Type:    ServerTypeRegistry,
				Source:  "http://example.com/v0/servers/my-server/versions/latest",
				Secrets: "default",
				Snapshot: &ServerSnapshot{
					Server: catalog.Server{
						Name:        "my-server",
						Type:        "server",
						Image:       "ghcr.io/example/my-server:latest",
						Description: "Test MCP server",
					},
				},
			}},
			expectedVersion: "latest",
		},
		{
			name:  "https registry",
			input: "https://example.com/v0/servers/my-server",
			expected: []Server{{
				Type:    ServerTypeRegistry,
				Source:  "https://example.com/v0/servers/my-server/versions/latest",
				Secrets: "default",
				Snapshot: &ServerSnapshot{
					Server: catalog.Server{
						Name:        "my-server",
						Type:        "server",
						Image:       "ghcr.io/example/my-server:latest",
						Description: "Test MCP server",
					},
				},
			}},
			expectedVersion: "latest",
		},
		{
			name:  "specific version registry",
			input: "https://example.com/v0/servers/my-server/versions/0.1.0",
			expected: []Server{{
				Type:    ServerTypeRegistry,
				Source:  "https://example.com/v0/servers/my-server/versions/0.1.0",
				Secrets: "default",
				Snapshot: &ServerSnapshot{
					Server: catalog.Server{
						Name:        "my-server",
						Type:        "server",
						Image:       "ghcr.io/example/my-server:0.1.0",
						Description: "Test MCP server",
					},
				},
			}},
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
			dao := setupTestDB(t)

			serverResponse := v0.ServerResponse{
				Server: v0.ServerJSON{
					Name:        "io.example/my-server",
					Description: "Test MCP server",
					Version:     tt.expectedVersion,
					Packages: []model.Package{
						{
							RegistryType: "oci",
							Identifier:   "ghcr.io/example/my-server:" + tt.expectedVersion,
							Transport: model.Transport{
								Type: "stdio",
							},
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

			ociService := mocks.NewMockOCIService(
				mocks.WithLocalImages([]mocks.MockImage{
					{
						Ref: "myimage:latest",
						Labels: map[string]string{
							"io.docker.server.metadata": "name: My Image",
						},
						DigestString: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					},
				}),
				mocks.WithRemoteImages([]mocks.MockImage{
					{
						Ref: "bobbarker/myimage:latest",
						Labels: map[string]string{
							"io.docker.server.metadata": "name: My Image",
						},
						DigestString: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					},
				}))

			server, err := ResolveServersFromString(t.Context(), registryClient, ociService, dao, tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, server)
			}
		})
	}
}

func TestResolveFile(t *testing.T) {
	// Files come from testdata/
	tests := []struct {
		name        string
		file        string
		input       string
		expected    []Server
		expectedErr string
	}{
		{
			name:  "valid yaml file",
			file:  "server.yaml",
			input: "file://testdata/server.yaml",
			expected: []Server{
				{
					Type:    ServerTypeImage,
					Image:   "myimage:latest",
					Secrets: "default",
					Snapshot: &ServerSnapshot{
						Server: catalog.Server{
							Name:        "my-mcp",
							Type:        "server",
							Image:       "myimage:latest",
							Description: "Server that runs my MCP code",
							Title:       "My MCP",
							Env: []catalog.Env{
								{
									Name:  "MODE",
									Value: "{{my-mcp.mode}}",
								},
							},
							Secrets: []catalog.Secret{
								{
									Name: "my-mcp.SECRET_KEY",
									Env:  "SECRET_KEY",
								},
							},
							Config: []any{
								map[string]any{
									"name":        "my-mcp",
									"description": "The configuration for the mcp server",
									"type":        "object",
									"properties": map[string]any{
										"mode": map[string]any{
											"type": "string",
										},
									},
									"required": []any{"mode"},
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "valid json file",
			file:  "server.json",
			input: "file://testdata/server.json",
			expected: []Server{
				{
					Type:    ServerTypeImage,
					Image:   "myimage:latest",
					Secrets: "default",
					Snapshot: &ServerSnapshot{
						Server: catalog.Server{
							Name:        "my-mcp",
							Type:        "server",
							Image:       "myimage:latest",
							Description: "Server that runs my MCP code",
							Title:       "My MCP",
							Env: []catalog.Env{
								{
									Name:  "MODE",
									Value: "{{my-mcp.mode}}",
								},
							},
							Secrets: []catalog.Secret{
								{
									Name: "my-mcp.SECRET_KEY",
									Env:  "SECRET_KEY",
								},
							},
							Config: []any{
								map[string]any{
									"name":        "my-mcp",
									"description": "The configuration for the mcp server",
									"type":        "object",
									"properties": map[string]any{
										"mode": map[string]any{
											"type": "string",
										},
									},
									"required": []any{"mode"},
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "valid full catalog yaml file",
			file:  "legacy-catalog.yaml",
			input: "file://testdata/legacy-catalog.yaml",
			expected: []Server{
				{
					Type:    ServerTypeImage,
					Image:   "myimage:latest",
					Secrets: "default",
					Snapshot: &ServerSnapshot{
						Server: catalog.Server{
							Name:        "my-mcp",
							Type:        "server",
							Image:       "myimage:latest",
							Description: "Server that runs my MCP code",
							Title:       "My MCP",
							Env: []catalog.Env{
								{
									Name:  "MODE",
									Value: "{{my-mcp.mode}}",
								},
							},
							Secrets: []catalog.Secret{
								{
									Name: "my-mcp.SECRET_KEY",
									Env:  "SECRET_KEY",
								},
							},
							Config: []any{
								map[string]any{
									"name":        "my-mcp",
									"description": "The configuration for the mcp server",
									"type":        "object",
									"properties": map[string]any{
										"mode": map[string]any{
											"type": "string",
										},
									},
									"required": []any{"mode"},
								},
							},
						},
					},
				},
			},
		},
		{
			name:        "invalid yaml file",
			file:        "invalid-yaml.yaml",
			input:       "file://testdata/invalid-yaml.yaml",
			expectedErr: "failed to unmarshal server",
		},
		{
			name:        "invalid type yaml file",
			file:        "invalid-type.yaml",
			input:       "file://testdata/invalid-type.yaml",
			expectedErr: "unsupported server type: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// write temp file to disk
			tempDir := t.TempDir()
			if tt.file != "" {
				content, err := testData.ReadFile("testdata/" + tt.file)
				require.NoError(t, err)
				tempFile := filepath.Join(tempDir, "testdata", tt.file)
				_ = os.MkdirAll(filepath.Dir(tempFile), 0o755)
				err = os.WriteFile(tempFile, content, 0o644)
				require.NoError(t, err)
				defer os.Remove(tempFile)
			}

			cwd, err := os.Getwd()
			require.NoError(t, err)
			defer os.Chdir(cwd) //nolint:errcheck
			err = os.Chdir(tempDir)
			require.NoError(t, err)

			server, err := ResolveServersFromString(t.Context(), mocks.NewMockRegistryAPIClient(), mocks.NewMockOCIService(), setupTestDB(t), tt.input)
			if tt.expectedErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, server)
			}
		})
	}
}

func TestResolveServerFromStringResolvesLatestVersion(t *testing.T) {
	dao := setupTestDB(t)

	serverResponse := v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.example/my-server",
			Description: "Test MCP server",
			Version:     "0.2.0",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/example/my-server:0.2.0",
					Transport: model.Transport{
						Type: "stdio",
					},
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
			Name:        "io.example/my-server",
			Description: "Test MCP server",
			Version:     "0.1.0",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/example/my-server:0.1.0",
					Transport: model.Transport{
						Type: "stdio",
					},
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

	server, err := ResolveServersFromString(t.Context(), registryClient, mocks.NewMockOCIService(), dao, "http://example.com/v0/servers/my-server")
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/v0/servers/my-server/versions/0.2.0", server[0].Source)
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
					Type:        "server",
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
					Type: "server",
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
					Type: "server",
				},
			},
		},
		{
			name: "image with full metadata including pulls and owner",
			server: Server{
				Type:  ServerTypeImage,
				Image: "testimage:v1.0",
			},
			labels: map[string]string{
				"io.docker.server.metadata": `name: GitHub Server
type: server
image: testimage:v1.0
description: Official GitHub MCP Server
title: GitHub Official
icon: https://avatars.githubusercontent.com/u/9919?s=200&v=4
metadata:
  pulls: 42055
  githubStars: 24479
  category: devops
  tags:
    - github
    - devops
  license: MIT License
  owner: github`,
			},
			expectError: false,
			expected: &ServerSnapshot{
				Server: catalog.Server{
					Name:        "GitHub Server",
					Type:        "server",
					Image:       "testimage:v1.0",
					Description: "Official GitHub MCP Server",
					Title:       "GitHub Official",
					Icon:        "https://avatars.githubusercontent.com/u/9919?s=200&v=4",
					Metadata: &catalog.Metadata{
						Pulls:       42055,
						GithubStars: 24479,
						Category:    "devops",
						Tags:        []string{"github", "devops"},
						License:     "MIT License",
						Owner:       "github",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, d, hasDigest := strings.Cut(tt.server.Image, "@")
			var ociService oci.Service
			if hasDigest {
				ociService = mocks.NewMockOCIService(mocks.WithRemoteImages([]mocks.MockImage{
					{
						Ref:          r,
						Labels:       tt.labels,
						DigestString: d,
					},
				}))
			} else {
				ociService = mocks.NewMockOCIService(mocks.WithLocalImages([]mocks.MockImage{
					{
						Ref:          r,
						Labels:       tt.labels,
						DigestString: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					},
				}))
			}
			ctx := t.Context()

			snapshot, err := ResolveSnapshot(ctx, ociService, tt.server)

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
				if tt.expected.Server.Icon != "" {
					assert.Equal(t, tt.expected.Server.Icon, snapshot.Server.Icon)
				}
				if tt.expected.Server.Metadata != nil {
					require.NotNil(t, snapshot.Server.Metadata)
					assert.Equal(t, tt.expected.Server.Metadata.Pulls, snapshot.Server.Metadata.Pulls)
					assert.Equal(t, tt.expected.Server.Metadata.GithubStars, snapshot.Server.Metadata.GithubStars)
					assert.Equal(t, tt.expected.Server.Metadata.Category, snapshot.Server.Metadata.Category)
					assert.Equal(t, tt.expected.Server.Metadata.Tags, snapshot.Server.Metadata.Tags)
					assert.Equal(t, tt.expected.Server.Metadata.License, snapshot.Server.Metadata.License)
					assert.Equal(t, tt.expected.Server.Metadata.Owner, snapshot.Server.Metadata.Owner)
				}
			}
		})
	}
}
