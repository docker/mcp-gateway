package workingset

import (
	"testing"

	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/test/mocks"
)

func getMockOciService() oci.Service {
	return mocks.NewMockOCIService(mocks.WithLocalImages([]mocks.MockImage{
		{
			Ref: "myimage:latest",
			Labels: map[string]string{
				"io.docker.server.metadata": "name: My Image\ntype: server\nimage: myimage:latest",
			},
			DigestString: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
		{
			Ref: "anotherimage:v1.0",
			Labels: map[string]string{
				"io.docker.server.metadata": "name: Another Image\ntype: server\nimage: anotherimage:v1.0",
			},
			DigestString: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
	}))
}

func getMockRegistryClient() registryapi.Client {
	server1 := v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.example/server1",
			Description: "Test server 1",
			Version:     "0.1.0",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/example/server1:0.1.0",
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

	server2 := v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.example/server2",
			Description: "Test server 2",
			Version:     "0.1.0",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/example/server2:0.1.0",
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

	return mocks.NewMockRegistryAPIClient(mocks.WithServerListResponses(map[string]v0.ServerListResponse{
		"https://example.com/v0/servers/server1/versions": {
			Servers: []v0.ServerResponse{server1},
		},
		"https://example.com/v0/servers/server2/versions": {
			Servers: []v0.ServerResponse{server2},
		},
	}), mocks.WithServerResponses(map[string]v0.ServerResponse{
		"https://example.com/v0/servers/server1/versions/0.1.0": server1,
		"https://example.com/v0/servers/server2/versions/0.1.0": server2,
	}))
}

func TestCreateWithDockerImages(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "", "My Test Set", []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}, []string{})
	require.NoError(t, err)

	// Verify the working set was created
	dbSet, err := dao.GetWorkingSet(ctx, "my-test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Equal(t, "my-test-set", dbSet.ID)
	assert.Equal(t, "My Test Set", dbSet.Name)
	assert.Len(t, dbSet.Servers, 2)

	assert.Equal(t, "image", dbSet.Servers[0].Type)
	assert.Equal(t, "myimage:latest", dbSet.Servers[0].Image)

	assert.Equal(t, "image", dbSet.Servers[1].Type)
	assert.Equal(t, "anotherimage:v1.0", dbSet.Servers[1].Image)
}

func TestCreateWithRegistryServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "", "Registry Set", []string{
		"https://example.com/v0/servers/server1",
		"https://example.com/v0/servers/server2",
	}, []string{})
	require.NoError(t, err)

	// Verify the working set was created
	dbSet, err := dao.GetWorkingSet(ctx, "registry-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Len(t, dbSet.Servers, 2)

	assert.Equal(t, "registry", dbSet.Servers[0].Type)
	assert.Equal(t, "https://example.com/v0/servers/server1/versions/0.1.0", dbSet.Servers[0].Source)

	assert.Equal(t, "registry", dbSet.Servers[1].Type)
	assert.Equal(t, "https://example.com/v0/servers/server2/versions/0.1.0", dbSet.Servers[1].Source)
}

func TestCreateWithMixedServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "", "Mixed Set", []string{
		"docker://myimage:latest",
		"https://example.com/v0/servers/server1",
	}, []string{})
	require.NoError(t, err)

	// Verify the working set was created
	dbSet, err := dao.GetWorkingSet(ctx, "mixed-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Len(t, dbSet.Servers, 2)
	assert.Equal(t, "image", dbSet.Servers[0].Type)
	assert.Equal(t, "registry", dbSet.Servers[1].Type)
}

func TestCreateWithCustomId(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "custom-id", "Test Set", []string{
		"docker://myimage:latest",
	}, []string{})
	require.NoError(t, err)

	// Verify the working set was created with custom ID
	dbSet, err := dao.GetWorkingSet(ctx, "custom-id")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Equal(t, "custom-id", dbSet.ID)
	assert.Equal(t, "Test Set", dbSet.Name)
}

func TestCreateWithExistingId(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create first working set
	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-id", "Test Set 1", []string{
		"docker://myimage:latest",
	}, []string{})
	require.NoError(t, err)

	// Try to create another with the same ID
	err = Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-id", "Test Set 2", []string{
		"docker://anotherimage:latest",
	}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCreateGeneratesUniqueIds(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create first working set
	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "", "Test Set", []string{
		"docker://myimage:latest",
	}, []string{})
	require.NoError(t, err)

	// Create second with same name
	err = Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "", "Test Set", []string{
		"docker://anotherimage:v1.0",
	}, []string{})
	require.NoError(t, err)

	// Create third with same name
	err = Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "", "Test Set", []string{
		"docker://anotherimage:v1.0",
	}, []string{})
	require.NoError(t, err)

	// List all working sets
	sets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Len(t, sets, 3)

	// Verify IDs are unique
	ids := make(map[string]bool)
	for _, set := range sets {
		assert.False(t, ids[set.ID], "ID %s should be unique", set.ID)
		ids[set.ID] = true
	}

	// Verify ID pattern
	assert.Contains(t, ids, "test-set")
	assert.Contains(t, ids, "test-set-2")
	assert.Contains(t, ids, "test-set-3")
}

func TestCreateWithInvalidServerFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "", "Test Set", []string{
		"invalid-format",
	}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid server value")
}

func TestCreateWithEmptyName(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-id", "", []string{
		"docker://myimage:latest",
	}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile")
}

func TestCreateWithEmptyServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "", "Empty Set", []string{}, []string{})
	require.NoError(t, err)

	// Verify the working set was created with no servers
	dbSet, err := dao.GetWorkingSet(ctx, "empty-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Empty(t, dbSet.Servers)
}

func TestCreateAddsDefaultSecrets(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "", "Test Set", []string{
		"docker://myimage:latest",
	}, []string{})
	require.NoError(t, err)

	// Verify default secrets were added
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)

	assert.Len(t, dbSet.Secrets, 1)
	assert.Contains(t, dbSet.Secrets, "default")
	assert.Equal(t, "docker-desktop-store", dbSet.Secrets["default"].Provider)
}

func TestCreateNameWithSpecialCharacters(t *testing.T) {
	tests := []struct {
		name       string
		inputName  string
		expectedID string
	}{
		{
			name:       "name with spaces",
			inputName:  "My Test Set",
			expectedID: "my-test-set",
		},
		{
			name:       "name with special chars",
			inputName:  "Test@Set#123!",
			expectedID: "test-set-123-",
		},
		{
			name:       "name with multiple spaces",
			inputName:  "Test   Set",
			expectedID: "test-set",
		},
		{
			name:       "name with underscores",
			inputName:  "Test_Set_Name",
			expectedID: "test-set-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh database for each subtest to avoid ID conflicts
			dao := setupTestDB(t)
			ctx := t.Context()

			err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "", tt.inputName, []string{
				"docker://myimage:latest",
			}, []string{})
			require.NoError(t, err)

			// Verify the ID was generated correctly
			dbSet, err := dao.GetWorkingSet(ctx, tt.expectedID)
			require.NoError(t, err)
			require.NotNil(t, dbSet)
			assert.Equal(t, tt.expectedID, dbSet.ID)
		})
	}
}
