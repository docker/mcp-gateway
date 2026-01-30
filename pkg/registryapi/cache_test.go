package registryapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	registryapi "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/stretchr/testify/require"
)

func TestIsCacheValid(t *testing.T) {
	tests := []struct {
		name  string
		cache *ServerCache
		want  bool
	}{
		{
			name:  "nil cache is invalid",
			cache: nil,
			want:  false,
		},
		{
			name: "expired cache is invalid",
			cache: &ServerCache{
				ExpiresAt: time.Now().Add(-1 * time.Minute),
			},
			want: false,
		},
		{
			name: "future expiry is valid",
			cache: &ServerCache{
				ExpiresAt: time.Now().Add(30 * time.Minute),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isCacheValid(tt.cache))
		})
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	servers := []registryapi.ServerResponse{
		{
			Server: registryapi.ServerJSON{
				Name:        "io.example/test-server",
				Version:     "1.0.0",
				Description: "A test server",
			},
		},
		{
			Server: registryapi.ServerJSON{
				Name:        "io.example/another-server",
				Version:     "2.0.0",
				Description: "Another test server",
			},
		},
	}

	err := saveCache(servers)
	require.NoError(t, err)

	loaded, err := loadCache()
	require.NoError(t, err)
	require.NotNil(t, loaded)

	require.Len(t, loaded.Servers, 2)
	require.Equal(t, "io.example/test-server", loaded.Servers[0].Server.Name)
	require.Equal(t, "io.example/another-server", loaded.Servers[1].Server.Name)
	require.True(t, isCacheValid(loaded))
	require.WithinDuration(t, time.Now(), loaded.CachedAt, 5*time.Second)
	require.WithinDuration(t, time.Now().Add(DefaultCacheTTL), loaded.ExpiresAt, 5*time.Second)
}

func TestLoadCacheMissingFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	loaded, err := loadCache()
	require.Error(t, err)
	require.Nil(t, loaded)
}

func TestLoadCacheCorruptedJSON(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cacheDir := filepath.Join(tempDir, ".docker", "mcp", "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, CacheFileName), []byte("not-json"), 0o644))

	loaded, err := loadCache()
	require.Error(t, err)
	require.Nil(t, loaded)
}

func TestGetCachedServers(t *testing.T) {
	t.Run("returns nil when no cache exists", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("HOME", tempDir)

		servers, err := GetCachedServers()
		require.NoError(t, err)
		require.Nil(t, servers)
	})

	t.Run("returns nil for expired cache", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("HOME", tempDir)

		cacheDir := filepath.Join(tempDir, ".docker", "mcp", "cache")
		require.NoError(t, os.MkdirAll(cacheDir, 0o755))

		cache := ServerCache{
			Servers: []registryapi.ServerResponse{
				{Server: registryapi.ServerJSON{Name: "io.example/expired"}},
			},
			CachedAt:  time.Now().Add(-2 * time.Hour),
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}
		data, err := json.Marshal(cache)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, CacheFileName), data, 0o644))

		servers, err := GetCachedServers()
		require.NoError(t, err)
		require.Nil(t, servers)
	})

	t.Run("returns servers for valid cache", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("HOME", tempDir)

		cacheDir := filepath.Join(tempDir, ".docker", "mcp", "cache")
		require.NoError(t, os.MkdirAll(cacheDir, 0o755))

		cache := ServerCache{
			Servers: []registryapi.ServerResponse{
				{Server: registryapi.ServerJSON{Name: "io.example/valid"}},
			},
			CachedAt:  time.Now(),
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		data, err := json.Marshal(cache)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, CacheFileName), data, 0o644))

		servers, err := GetCachedServers()
		require.NoError(t, err)
		require.Len(t, servers, 1)
		require.Equal(t, "io.example/valid", servers[0].Server.Name)
	})

	t.Run("returns nil for corrupted cache", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("HOME", tempDir)

		cacheDir := filepath.Join(tempDir, ".docker", "mcp", "cache")
		require.NoError(t, os.MkdirAll(cacheDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, CacheFileName), []byte("{bad"), 0o644))

		servers, err := GetCachedServers()
		require.NoError(t, err)
		require.Nil(t, servers)
	})
}

func TestCacheServers(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	servers := []registryapi.ServerResponse{
		{Server: registryapi.ServerJSON{Name: "io.example/cached"}},
	}

	err := CacheServers(servers)
	require.NoError(t, err)

	// Verify file was created
	cachePath := filepath.Join(tempDir, ".docker", "mcp", "cache", CacheFileName)
	_, err = os.Stat(cachePath)
	require.NoError(t, err)

	// Verify contents
	retrieved, err := GetCachedServers()
	require.NoError(t, err)
	require.Len(t, retrieved, 1)
	require.Equal(t, "io.example/cached", retrieved[0].Server.Name)
}

func TestInvalidateCache(t *testing.T) {
	t.Run("removes existing cache", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("HOME", tempDir)

		err := CacheServers([]registryapi.ServerResponse{
			{Server: registryapi.ServerJSON{Name: "io.example/to-remove"}},
		})
		require.NoError(t, err)

		err = InvalidateCache()
		require.NoError(t, err)

		cachePath := filepath.Join(tempDir, ".docker", "mcp", "cache", CacheFileName)
		_, err = os.Stat(cachePath)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("no error when cache does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("HOME", tempDir)

		err := InvalidateCache()
		require.NoError(t, err)
	})
}
