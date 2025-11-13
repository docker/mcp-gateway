package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/user"
)

const (
	DockerCatalogFilename = "docker-mcp.yaml"
)

func Get(ctx context.Context) (Catalog, error) {
	return ReadFrom(ctx, []string{DockerCatalogFilename})
}

func ReadFrom(ctx context.Context, fileOrURLs []string) (Catalog, error) {
	mergedServers := map[string]Server{}

	for _, fileOrURL := range fileOrURLs {
		servers, _, _, err := readMCPServers(ctx, fileOrURL)
		if err != nil {
			return Catalog{}, err
		}

		// Merge servers into the combined map, checking for overlaps
		for key, server := range servers {
			if _, exists := mergedServers[key]; exists {
				log.Printf("Warning: overlapping key '%s' found in catalog '%s', overwriting previous value", key, fileOrURL)
			}
			mergedServers[key] = server
		}
	}

	return Catalog{
		Servers: mergedServers,
	}, nil
}

func ReadOne(ctx context.Context, fileOrURL string) (Catalog, string, string, error) {
	servers, name, displayName, err := readMCPServers(ctx, fileOrURL)
	if err != nil {
		return Catalog{}, "", "", err
	}
	return Catalog{
		Servers: servers,
	}, name, displayName, nil
}

func readMCPServers(ctx context.Context, fileOrURL string) (map[string]Server, string, string, error) {
	buf, err := readFileOrURL(ctx, fileOrURL)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Server{}, "", "", nil
		}
		return nil, "", "", err
	}

	var topLevel topLevel
	if err := yaml.Unmarshal(buf, &topLevel); err != nil {
		return nil, "", "", err
	}

	return topLevel.Registry, topLevel.Name, topLevel.DisplayName, nil
}

func readFileOrURL(ctx context.Context, fileOrURL string) ([]byte, error) {
	switch {
	case isURL(fileOrURL):
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileOrURL, nil)
		if err != nil {
			return nil, err
		}

		client := &http.Client{
			Transport: http.DefaultTransport,
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch URL: %s, status: %s", fileOrURL, resp.Status)
		}

		return io.ReadAll(resp.Body)

	case filepath.IsAbs(fileOrURL) || strings.HasPrefix(fileOrURL, "./"):
		buf, err := os.ReadFile(fileOrURL)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return buf, nil

	default:
		homeDir, err := user.HomeDir()
		if err != nil {
			return nil, err
		}

		path := filepath.Join(homeDir, ".docker", "mcp", "catalogs", fileOrURL)

		buf, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return buf, nil
	}
}

func isURL(fileOrURL string) bool {
	return strings.HasPrefix(fileOrURL, "http://") || strings.HasPrefix(fileOrURL, "https://")
}

// GetWithOptions loads catalogs with enhanced options for configured catalogs and additional catalogs
func GetWithOptions(ctx context.Context, useConfigured bool, additionalCatalogs []string) (Catalog, error) {
	catalogPaths := []string{DockerCatalogFilename}

	// Add configured catalogs if enabled
	if useConfigured {
		configuredCatalogs, err := getConfiguredCatalogs()
		if err != nil {
			log.Printf("Warning: failed to load configured catalogs: %v", err)
		} else {
			catalogPaths = append(catalogPaths, configuredCatalogs...)
		}
	}

	// Add any additional catalogs specified via CLI
	if len(additionalCatalogs) > 0 {
		catalogPaths = append(catalogPaths, additionalCatalogs...)
	}

	// Remove duplicates while preserving order
	catalogPaths = removeDuplicates(catalogPaths)

	return ReadFrom(ctx, catalogPaths)
}

// removeDuplicates removes duplicate strings while preserving order (first occurrence wins)
func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

// getConfiguredCatalogs reads the catalog registry and returns the list of configured catalog files
func getConfiguredCatalogs() ([]string, error) {
	homeDir, err := user.HomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	catalogRegistryPath := filepath.Join(homeDir, ".docker", "mcp", "catalog.json")

	// Read the catalog registry file
	data, err := os.ReadFile(catalogRegistryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // No configured catalogs, return empty list
		}
		return nil, fmt.Errorf("failed to read catalog registry: %w", err)
	}

	// Parse the registry
	var registry struct {
		Catalogs map[string]struct {
			DisplayName string `json:"displayName"`
			URL         string `json:"url"`
			LastUpdate  string `json:"lastUpdate"`
		} `json:"catalogs"`
	}

	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse catalog registry: %w", err)
	}

	// Convert catalog names to file paths
	var catalogFiles []string
	for catalogName := range registry.Catalogs {
		catalogFiles = append(catalogFiles, catalogName+".yaml")
	}

	return catalogFiles, nil
}
