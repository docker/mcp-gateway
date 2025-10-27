package workingset

import (
	"path/filepath"
	"testing"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) (db.DAO, string) {
	t.Helper()

	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	dao, err := db.New(db.WithDatabaseFile(dbFile))
	require.NoError(t, err)

	return dao, dbFile
}

func TestNewFromDb(t *testing.T) {
	dbSet := &db.WorkingSet{
		ID:   "test-id",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:   "registry",
				Source: "https://example.com/server",
				Config: map[string]interface{}{"key": "value"},
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
	assert.Equal(t, map[string]interface{}{"key": "value"}, workingSet.Servers[0].Config)
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
				Config: map[string]interface{}{"key": "value"},
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
	assert.Equal(t, map[string]interface{}{"key": "value"}, dbSet.Servers[0].Config)
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
				Config:  map[string]interface{}{"key": "value"},
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
		existingIds []string
		expectedId  string
	}{
		{
			name:       "simple name",
			inputName:  "MyWorkingSet",
			expectedId: "myworkingset",
		},
		{
			name:       "name with spaces",
			inputName:  "My Working Set",
			expectedId: "my-working-set",
		},
		{
			name:       "name with special characters",
			inputName:  "My@Working#Set!",
			expectedId: "my-working-set-",
		},
		{
			name:        "name with collision",
			inputName:   "test",
			existingIds: []string{"test"},
			expectedId:  "test-2",
		},
		{
			name:        "name with multiple collisions",
			inputName:   "test",
			existingIds: []string{"test", "test-2", "test-3"},
			expectedId:  "test-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh database for each subtest to avoid ID conflicts
			dao, _ := setupTestDB(t)
			ctx := t.Context()

			// Setup: create existing working sets
			for _, id := range tt.existingIds {
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
			assert.Equal(t, tt.expectedId, id)
		})
	}
}

func TestResolveServerFromString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    Server
		expectError bool
	}{
		{
			name:  "docker image",
			input: "docker://myimage:latest",
			expected: Server{
				Type:  ServerTypeImage,
				Image: "myimage:latest",
			},
		},
		{
			name:  "http registry",
			input: "http://example.com/server",
			expected: Server{
				Type:   ServerTypeRegistry,
				Source: "http://example.com/server",
			},
		},
		{
			name:  "https registry",
			input: "https://example.com/server",
			expected: Server{
				Type:   ServerTypeRegistry,
				Source: "https://example.com/server",
			},
		},
		{
			name:        "invalid format",
			input:       "invalid-format",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := resolveServerFromString(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, server)
			}
		})
	}
}
