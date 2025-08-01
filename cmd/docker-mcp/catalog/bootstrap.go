package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Bootstrap creates a starter catalog file with Docker and Docker Hub server entries as examples
func Bootstrap(ctx context.Context, outputPath string) error {
	// Check if output file already exists
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("file %q already exists - will not overwrite", outputPath)
	}

	// Load Docker catalog configuration
	_, err := ReadConfigWithDefaultCatalog(ctx)
	if err != nil {
		return fmt.Errorf("failed to load Docker catalog config: %w", err)
	}

	// Read Docker catalog YAML data
	dockerCatalogData, err := ReadCatalogFile(DockerCatalogName)
	if err != nil {
		return fmt.Errorf("failed to read Docker catalog: %w", err)
	}

	// Parse the Docker catalog to extract server entries
	var dockerCatalog map[string]any
	if err := yaml.Unmarshal(dockerCatalogData, &dockerCatalog); err != nil {
		return fmt.Errorf("failed to parse Docker catalog: %w", err)
	}

	// Extract registry section
	registryInterface, ok := dockerCatalog["registry"]
	if !ok {
		return fmt.Errorf("docker catalog missing 'registry' section")
	}

	registry, ok := registryInterface.(map[string]any)
	if !ok {
		return fmt.Errorf("docker catalog 'registry' section is not a map")
	}

	// Extract Docker and Docker Hub servers
	dockerHubServer, hasDockerHub := registry[DockerHubServerName]
	dockerCLIServer, hasDockerCLI := registry[DockerCLIServerName]

	if !hasDockerHub {
		return fmt.Errorf("docker catalog missing '%s' server", DockerHubServerName)
	}
	if !hasDockerCLI {
		return fmt.Errorf("docker catalog missing '%s' server", DockerCLIServerName)
	}

	// Create bootstrap catalog with just the Docker servers
	bootstrapCatalog := map[string]any{
		"registry": map[string]any{
			DockerHubServerName: dockerHubServer,
			DockerCLIServerName: dockerCLIServer,
		},
	}

	// Marshal to YAML
	bootstrapData, err := yaml.Marshal(bootstrapCatalog)
	if err != nil {
		return fmt.Errorf("failed to marshal bootstrap catalog: %w", err)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write the bootstrap catalog file
	if err := os.WriteFile(outputPath, bootstrapData, 0o644); err != nil {
		return fmt.Errorf("failed to write bootstrap catalog file: %w", err)
	}

	return nil
}
