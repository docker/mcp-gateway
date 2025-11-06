package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/config"
)

func TestReset(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Setup test environment with user catalogs
	setupTestCatalogsForReset(t, tempHome)

	ctx := t.Context()

	// Verify initial state - should have docker-mcp and user catalogs
	initialCfg, err := ReadConfig()
	require.NoError(t, err)
	assert.Len(t, initialCfg.Catalogs, 2, "should have 2 catalogs initially (docker-mcp + my-catalog)")
	assert.Contains(t, initialCfg.Catalogs, DockerCatalogName, "should have docker-mcp catalog")
	assert.Contains(t, initialCfg.Catalogs, "my-catalog", "should have my-catalog")

	// Verify catalog files exist
	catalogsDir, err := config.FilePath("catalogs")
	require.NoError(t, err)
	dockerCatalogFile := filepath.Join(catalogsDir, "docker-mcp.yaml")
	userCatalogFile := filepath.Join(catalogsDir, "my-catalog.yaml")
	assert.FileExists(t, dockerCatalogFile, "docker-mcp.yaml should exist")
	assert.FileExists(t, userCatalogFile, "my-catalog.yaml should exist")

	// Execute reset
	err = Reset(ctx)
	require.NoError(t, err, "reset should succeed")

	// Verify catalogs directory was recreated (Import creates it)
	assert.DirExists(t, catalogsDir, "catalogs directory should exist after reset")

	// Verify user catalog file is removed
	assert.NoFileExists(t, userCatalogFile, "my-catalog.yaml should be removed")

	// Verify docker-mcp catalog is automatically reimported
	assert.FileExists(t, dockerCatalogFile, "docker-mcp.yaml should be automatically reimported")

	// Read config and verify docker-mcp catalog exists
	finalCfg, err := ReadConfig()
	require.NoError(t, err)
	assert.Len(t, finalCfg.Catalogs, 1, "should have exactly 1 catalog after reset (docker-mcp)")
	assert.Contains(t, finalCfg.Catalogs, DockerCatalogName, "should have docker-mcp catalog")
	assert.NotContains(t, finalCfg.Catalogs, "my-catalog", "should not have my-catalog after reset")

	// Verify docker-mcp catalog has proper metadata
	dockerCatalog := finalCfg.Catalogs[DockerCatalogName]
	assert.NotEmpty(t, dockerCatalog.DisplayName, "docker-mcp catalog should have display name")
}

func TestResetEmptyCatalogs(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Create minimal directory structure without any catalogs
	mcpDir := filepath.Join(tempHome, ".docker", "mcp")
	err := os.MkdirAll(mcpDir, 0o755)
	require.NoError(t, err)

	ctx := context.Background()

	// Execute reset on empty state
	err = Reset(ctx)
	require.NoError(t, err, "reset should succeed even with no existing catalogs")

	// Verify docker-mcp catalog is imported
	cfg, err := ReadConfig()
	require.NoError(t, err)
	assert.Len(t, cfg.Catalogs, 1, "should have exactly 1 catalog after reset (docker-mcp)")
	assert.Contains(t, cfg.Catalogs, DockerCatalogName, "should have docker-mcp catalog")
}

// Helper function to set up test catalogs for reset testing
func setupTestCatalogsForReset(t *testing.T, homeDir string) {
	t.Helper()

	// Create .docker/mcp directory structure
	mcpDir := filepath.Join(homeDir, ".docker", "mcp")
	catalogsDir := filepath.Join(mcpDir, "catalogs")
	err := os.MkdirAll(catalogsDir, 0o755)
	require.NoError(t, err)

	// Create catalog.json registry with docker-mcp and a user catalog
	catalogRegistry := `{
  "catalogs": {
    "docker-mcp": {
      "displayName": "Docker MCP Catalog",
      "url": "` + DockerCatalogURLV2 + `",
      "lastUpdate": "2025-08-01T00:00:00Z"
    },
    "my-catalog": {
      "displayName": "My Custom Catalog",
      "url": "",
      "lastUpdate": "2025-08-01T00:00:00Z"
    }
  }
}`
	err = os.WriteFile(filepath.Join(mcpDir, "catalog.json"), []byte(catalogRegistry), 0o644)
	require.NoError(t, err)

	// Create docker-mcp.yaml (Docker catalog)
	dockerCatalog := `name: docker-mcp
displayName: Docker MCP Catalog
registry:
  dockerhub:
    description: "Docker Hub official MCP server."
    title: "Docker Hub"
    image: "mcp/dockerhub@sha256:test123"
    tools:
      - name: "search"
      - name: "getRepositoryInfo"
  docker:
    description: "Use the Docker CLI."
    title: "Docker"
    type: "poci"
    image: "docker@sha256:test456"
    tools:
      - name: "docker"`
	err = os.WriteFile(filepath.Join(catalogsDir, "docker-mcp.yaml"), []byte(dockerCatalog), 0o644)
	require.NoError(t, err)

	// Create my-catalog.yaml (user catalog)
	customCatalog := `name: my-catalog
displayName: My Custom Catalog
registry:
  custom-server:
    image: custom/test-server
    tools:
      - name: custom-tool
        description: "Custom Catalog Server"
        container:
          image: custom/test-server
          command: []`
	err = os.WriteFile(filepath.Join(catalogsDir, "my-catalog.yaml"), []byte(customCatalog), 0o644)
	require.NoError(t, err)
}
