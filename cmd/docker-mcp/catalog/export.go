package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/user"
)

// Export exports a configured catalog to a file
// This function only allows exporting user-managed catalogs, not the Docker catalog
func Export(_ context.Context, catalogName, outputPath string) error {
	// Validate that we're not trying to export the Docker catalog
	if catalogName == DockerCatalogName || catalogName == DockerCatalogFilename {
		return fmt.Errorf("cannot export the Docker MCP catalog as it is managed by Docker")
	}

	// Get configured catalogs to verify the catalog exists
	configuredCatalogs, err := getConfiguredCatalogs()
	if err != nil {
		return fmt.Errorf("failed to read configured catalogs: %w", err)
	}

	// Check if the catalog exists in the configured catalogs
	catalogFileName := catalogName + ".yaml"
	found := false
	for _, configuredCatalog := range configuredCatalogs {
		if configuredCatalog == catalogFileName {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("catalog '%s' not found in configured catalogs", catalogName)
	}

	// Read the catalog file
	homeDir, err := user.HomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	catalogPath := filepath.Join(homeDir, ".docker", "mcp", "catalogs", catalogFileName)
	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		return fmt.Errorf("failed to read catalog file: %w", err)
	}

	// Parse the catalog to validate it
	var catalogContent catalog.Catalog
	if err := yaml.Unmarshal(catalogData, &catalogContent); err != nil {
		return fmt.Errorf("failed to parse catalog: %w", err)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write the catalog to the output file
	if err := os.WriteFile(outputPath, catalogData, 0o644); err != nil {
		return fmt.Errorf("failed to write export file: %w", err)
	}

	fmt.Printf("Catalog '%s' exported to '%s'\n", catalogName, outputPath)
	return nil
}

// Helper function to get configured catalogs (same logic as in internal/catalog)
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
