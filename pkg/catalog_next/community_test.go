package catalognext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func TestIsAPIRegistry(t *testing.T) {
	tests := []struct {
		name   string
		ref    string
		expect bool
	}{
		{"matches hostname", "registry.modelcontextprotocol.io", true},
		{"matches with latest tag", "registry.modelcontextprotocol.io:latest", true},
		{"matches with custom tag", "registry.modelcontextprotocol.io:v1", true},
		{"rejects unknown hostname", "registry.example.com", false},
		{"rejects OCI reference", "mcp/docker-mcp-catalog:latest", false},
		{"rejects empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, IsAPIRegistry(tt.ref))
		})
	}
}

func TestGetRegistryURL(t *testing.T) {
	t.Run("returns URL for known registry", func(t *testing.T) {
		url := GetRegistryURL("registry.modelcontextprotocol.io")
		assert.Equal(t, "https://registry.modelcontextprotocol.io", url)
	})

	t.Run("returns URL with tag suffix", func(t *testing.T) {
		url := GetRegistryURL("registry.modelcontextprotocol.io:latest")
		assert.Equal(t, "https://registry.modelcontextprotocol.io", url)
	})

	t.Run("returns empty for unknown registry", func(t *testing.T) {
		url := GetRegistryURL("registry.example.com")
		assert.Empty(t, url)
	})
}

func TestWriteCommunityToDatabase(t *testing.T) {
	t.Run("image server stored as ServerTypeImage", func(t *testing.T) {
		dao := setupTestDB(t)
		ctx := t.Context()

		servers := map[string]catalog.Server{
			"test-server": {
				Name:  "test-server",
				Type:  "server",
				Image: "ghcr.io/example/test:latest",
			},
		}

		err := writeCommunityToDatabase(ctx, dao, "registry.modelcontextprotocol.io:latest", servers)
		require.NoError(t, err)

		dbCatalog, err := dao.GetCatalog(ctx, "registry.modelcontextprotocol.io:latest")
		require.NoError(t, err)

		cat := NewFromDb(dbCatalog)
		require.Len(t, cat.Servers, 1)
		assert.Equal(t, workingset.ServerTypeImage, cat.Servers[0].Type)
		assert.Equal(t, "ghcr.io/example/test:latest", cat.Servers[0].Image)
	})

	t.Run("remote server stored as ServerTypeRemote", func(t *testing.T) {
		dao := setupTestDB(t)
		ctx := t.Context()

		servers := map[string]catalog.Server{
			"remote-server": {
				Name: "remote-server",
				Type: "remote",
				Remote: catalog.Remote{
					URL:       "https://api.example.com/mcp",
					Transport: "streamable-http",
				},
			},
		}

		err := writeCommunityToDatabase(ctx, dao, "registry.modelcontextprotocol.io:latest", servers)
		require.NoError(t, err)

		dbCatalog, err := dao.GetCatalog(ctx, "registry.modelcontextprotocol.io:latest")
		require.NoError(t, err)

		cat := NewFromDb(dbCatalog)
		require.Len(t, cat.Servers, 1)
		assert.Equal(t, workingset.ServerTypeRemote, cat.Servers[0].Type)
		assert.Equal(t, "https://api.example.com/mcp", cat.Servers[0].Endpoint)
	})

	t.Run("mixed server types stored correctly", func(t *testing.T) {
		dao := setupTestDB(t)
		ctx := t.Context()

		servers := map[string]catalog.Server{
			"image-server": {
				Name:  "image-server",
				Type:  "server",
				Image: "ghcr.io/example/img:latest",
			},
			"remote-server": {
				Name: "remote-server",
				Type: "remote",
				Remote: catalog.Remote{
					URL:       "https://api.example.com/mcp",
					Transport: "streamable-http",
				},
			},
		}

		err := writeCommunityToDatabase(ctx, dao, "registry.modelcontextprotocol.io:latest", servers)
		require.NoError(t, err)

		dbCatalog, err := dao.GetCatalog(ctx, "registry.modelcontextprotocol.io:latest")
		require.NoError(t, err)

		cat := NewFromDb(dbCatalog)
		require.Len(t, cat.Servers, 2)

		// Servers are sorted by name
		assert.Equal(t, workingset.ServerTypeImage, cat.Servers[0].Type)
		assert.Equal(t, "ghcr.io/example/img:latest", cat.Servers[0].Image)
		assert.Equal(t, workingset.ServerTypeRemote, cat.Servers[1].Type)
		assert.Equal(t, "https://api.example.com/mcp", cat.Servers[1].Endpoint)
	})

	t.Run("source prefix is registry:", func(t *testing.T) {
		dao := setupTestDB(t)
		ctx := t.Context()

		err := writeCommunityToDatabase(ctx, dao, "registry.modelcontextprotocol.io:latest", map[string]catalog.Server{})
		require.NoError(t, err)

		dbCatalog, err := dao.GetCatalog(ctx, "registry.modelcontextprotocol.io:latest")
		require.NoError(t, err)
		assert.Equal(t, "registry:registry.modelcontextprotocol.io:latest", dbCatalog.Source)
	})
}

func TestPullCommunityUnknownRegistry(t *testing.T) {
	tests := []struct {
		name   string
		refStr string
	}{
		{
			name:   "completely unknown hostname",
			refStr: "registry.unknown.io",
		},
		{
			name:   "unknown hostname with tag",
			refStr: "registry.unknown.io:latest",
		},
		{
			name:   "OCI-style reference",
			refStr: "docker/mcp-catalog:latest",
		},
		{
			name:   "empty string",
			refStr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dao := setupTestDB(t)
			ctx := t.Context()

			result, err := PullCommunity(ctx, dao, tt.refStr, DefaultPullCommunityOptions())
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "unknown community registry")
		})
	}
}

func TestAppendIfMissing(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		val      string
		expected []string
	}{
		{
			name:     "appends to nil slice",
			slice:    nil,
			val:      "a",
			expected: []string{"a"},
		},
		{
			name:     "appends to empty slice",
			slice:    []string{},
			val:      "a",
			expected: []string{"a"},
		},
		{
			name:     "appends new value",
			slice:    []string{"a", "b"},
			val:      "c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "does not duplicate existing value",
			slice:    []string{"a", "b", "c"},
			val:      "b",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "does not duplicate first value",
			slice:    []string{"a", "b"},
			val:      "a",
			expected: []string{"a", "b"},
		},
		{
			name:     "does not duplicate last value",
			slice:    []string{"a", "b"},
			val:      "b",
			expected: []string{"a", "b"},
		},
		{
			name:     "handles single element slice with same value",
			slice:    []string{"community"},
			val:      "community",
			expected: []string{"community"},
		},
		{
			name:     "handles single element slice with different value",
			slice:    []string{"official"},
			val:      "community",
			expected: []string{"official", "community"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendIfMissing(tt.slice, tt.val)
			assert.Equal(t, tt.expected, result)
		})
	}
}
