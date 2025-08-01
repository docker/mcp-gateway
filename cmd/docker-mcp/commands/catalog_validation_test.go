package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
)

func TestDockerCatalogProtection(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
		}
	}()

	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}

	// Create the MCP directory structure
	mcpDir := filepath.Join(tempHome, ".docker", "mcp")
	catalogsDir := filepath.Join(mcpDir, "catalogs")
	require.NoError(t, os.MkdirAll(catalogsDir, 0o755))

	// Initialize the catalog system
	ctx := context.Background()
	require.NoError(t, catalog.Init(ctx))

	t.Run("TestCreateDockerCatalogPrevented", func(t *testing.T) {
		err := catalog.Create(catalog.DockerCatalogName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot create catalog 'docker-mcp' as it is reserved for Docker's official catalog")
	})

	t.Run("TestRemoveDockerCatalogPrevented", func(t *testing.T) {
		err := catalog.Rm(catalog.DockerCatalogName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot remove catalog 'docker-mcp' as it is managed by Docker")
	})

	t.Run("TestForkToDockerCatalogPrevented", func(t *testing.T) {
		// First create a source catalog to fork from
		require.NoError(t, catalog.Create("source-catalog"))

		err := catalog.Fork("source-catalog", catalog.DockerCatalogName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot create catalog 'docker-mcp' as it is reserved for Docker's official catalog")
	})

	t.Run("TestForkFromDockerCatalogAllowed", func(t *testing.T) {
		// Create a minimal docker-mcp.yaml file to fork from
		dockerCatalog := `
registry:
  test-server:
    title: "Test Server"
    description: "A test server"
    image: "test/server:latest"
`
		dockerCatalogPath := filepath.Join(catalogsDir, "docker-mcp.yaml")
		require.NoError(t, os.WriteFile(dockerCatalogPath, []byte(dockerCatalog), 0o644))

		// Forking FROM Docker catalog should work
		err := catalog.Fork(catalog.DockerCatalogName, "my-docker-fork")
		require.NoError(t, err)

		// Verify the fork was created
		cfg, err := catalog.ReadConfig()
		require.NoError(t, err)
		_, exists := cfg.Catalogs["my-docker-fork"]
		assert.True(t, exists)
	})

	t.Run("TestAddToDockerCatalogPrevented", func(t *testing.T) {
		// Create a source catalog file
		sourceCatalog := `
registry:
  source-server:
    title: "Source Server"
    description: "A source server"
    image: "source/server:latest"
`
		sourcePath := filepath.Join(catalogsDir, "source.yaml")
		require.NoError(t, os.WriteFile(sourcePath, []byte(sourceCatalog), 0o644))

		// Try to add to Docker catalog
		args := catalog.ParseAddArgs(catalog.DockerCatalogName, "test-server", sourcePath)
		err := catalog.ValidateArgs(*args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot add servers to catalog 'docker-mcp' as it is managed by Docker")
	})

	t.Run("TestExportDockerCatalogPrevented", func(t *testing.T) {
		outputPath := filepath.Join(tempHome, "exported-docker.yaml")
		err := catalog.Export(ctx, catalog.DockerCatalogName, outputPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot export the Docker MCP catalog as it is managed by Docker")
	})

	t.Run("TestExportDockerCatalogFilenamePrevented", func(t *testing.T) {
		outputPath := filepath.Join(tempHome, "exported-docker.yaml")
		err := catalog.Export(ctx, catalog.DockerCatalogFilename, outputPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot export the Docker MCP catalog as it is managed by Docker")
	})

	t.Run("TestNormalCatalogOperationsStillWork", func(t *testing.T) {
		// Create a normal catalog
		err := catalog.Create("normal-catalog")
		require.NoError(t, err)

		// Verify it exists
		cfg, err := catalog.ReadConfig()
		require.NoError(t, err)
		_, exists := cfg.Catalogs["normal-catalog"]
		assert.True(t, exists)

		// Fork it
		err = catalog.Fork("normal-catalog", "forked-catalog")
		require.NoError(t, err)

		// Add to it (create source file first)
		sourceCatalog := `
registry:
  normal-server:
    title: "Normal Server"
    description: "A normal server"
    image: "normal/server:latest"
`
		sourcePath := filepath.Join(catalogsDir, "normal-source.yaml")
		require.NoError(t, os.WriteFile(sourcePath, []byte(sourceCatalog), 0o644))

		args := catalog.ParseAddArgs("normal-catalog", "normal-server", sourcePath)
		err = catalog.ValidateArgs(*args)
		require.NoError(t, err)

		err = catalog.Add(*args, false)
		require.NoError(t, err)

		// Export it
		outputPath := filepath.Join(tempHome, "exported-normal.yaml")
		err = catalog.Export(ctx, "normal-catalog", outputPath)
		require.NoError(t, err)

		// Verify export file exists
		_, err = os.Stat(outputPath)
		require.NoError(t, err)

		// Remove it
		err = catalog.Rm("normal-catalog")
		require.NoError(t, err)

		// Verify it's gone
		cfg, err = catalog.ReadConfig()
		require.NoError(t, err)
		_, exists = cfg.Catalogs["normal-catalog"]
		assert.False(t, exists)
	})
}
