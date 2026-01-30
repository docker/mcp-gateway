package registryapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	registryapi "github.com/modelcontextprotocol/registry/pkg/api/v0"

	"github.com/docker/mcp-gateway/pkg/user"
)

const (
	// DefaultCacheTTL is the default time-to-live for cached server lists
	DefaultCacheTTL = 1 * time.Hour
	// CacheFileName is the name of the cache file
	CacheFileName = "community-registry-cache.json"
)

// ServerCache represents the cached server list with metadata
type ServerCache struct {
	Servers   []registryapi.ServerResponse `json:"servers"`
	CachedAt  time.Time                    `json:"cachedAt"`
	ExpiresAt time.Time                    `json:"expiresAt"`
}

// getCacheFilePath returns the path to the cache file
func getCacheFilePath() (string, error) {
	homeDir, err := user.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".docker", "mcp", "cache", CacheFileName), nil
}

// loadCache loads the cached server list from disk
func loadCache() (*ServerCache, error) {
	cachePath, err := getCacheFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var cache ServerCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

// saveCache saves the server list to disk cache
func saveCache(servers []registryapi.ServerResponse) error {
	cachePath, err := getCacheFilePath()
	if err != nil {
		return err
	}

	// Ensure cache directory exists
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	now := time.Now()
	cache := ServerCache{
		Servers:   servers,
		CachedAt:  now,
		ExpiresAt: now.Add(DefaultCacheTTL),
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0o644)
}

// isCacheValid checks if the cache is still valid
func isCacheValid(cache *ServerCache) bool {
	if cache == nil {
		return false
	}
	return time.Now().Before(cache.ExpiresAt)
}

// GetCachedServers returns cached servers if available and valid, nil otherwise
func GetCachedServers() ([]registryapi.ServerResponse, error) {
	cache, err := loadCache()
	if err != nil {
		// Cache doesn't exist or is corrupted, return nil (not an error)
		return nil, nil
	}

	if !isCacheValid(cache) {
		return nil, nil
	}

	return cache.Servers, nil
}

// CacheServers saves servers to the cache
func CacheServers(servers []registryapi.ServerResponse) error {
	return saveCache(servers)
}

// InvalidateCache removes the cache file
func InvalidateCache() error {
	cachePath, err := getCacheFilePath()
	if err != nil {
		return err
	}

	err = os.Remove(cachePath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
