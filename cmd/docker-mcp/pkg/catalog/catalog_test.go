package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogGetWithConfigured(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Create test catalog registry and configured catalog
	setupTestCatalogs(t, tempHome)

	ctx := context.Background()

	// Test loading with configured catalogs enabled
	catalog, err := GetWithOptions(ctx, true, nil)
	require.NoError(t, err)

	// Should contain servers from both Docker and configured catalogs
	assert.Contains(t, catalog.Servers, "docker-server", "should contain Docker catalog server")
	assert.Contains(t, catalog.Servers, "custom-server", "should contain configured catalog server")
}

func TestCatalogGetWithoutConfigured(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Create test catalog registry and configured catalog
	setupTestCatalogs(t, tempHome)

	ctx := context.Background()

	// Test loading with configured catalogs disabled (current behavior)
	catalog, err := GetWithOptions(ctx, false, nil)
	require.NoError(t, err)

	// Should only contain servers from Docker catalog
	assert.Contains(t, catalog.Servers, "docker-server", "should contain Docker catalog server")
	assert.NotContains(t, catalog.Servers, "custom-server", "should NOT contain configured catalog server")
}

func TestGetConfiguredCatalogsSuccess(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Create test catalog registry
	setupTestCatalogs(t, tempHome)

	// Test reading configured catalogs
	catalogFiles, err := getConfiguredCatalogs()
	require.NoError(t, err)

	// Should return the configured catalog files
	assert.Contains(t, catalogFiles, "my-catalog.yaml")
}

func TestGetConfiguredCatalogsMissing(t *testing.T) {
	// Create temporary home directory with no catalog.json
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Test reading missing catalog registry
	catalogFiles, err := getConfiguredCatalogs()

	// Should handle gracefully - either return empty list or error
	if err != nil {
		assert.Contains(t, err.Error(), "catalog.json")
	} else {
		assert.Empty(t, catalogFiles)
	}
}

func TestGetConfiguredCatalogsCorrupt(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Create corrupted catalog.json
	mcpDir := filepath.Join(tempHome, ".docker", "mcp")
	err := os.MkdirAll(mcpDir, 0o755)
	require.NoError(t, err)

	catalogFile := filepath.Join(mcpDir, "catalog.json")
	err = os.WriteFile(catalogFile, []byte("invalid json{"), 0o644)
	require.NoError(t, err)

	// Test reading corrupted catalog registry
	catalogFiles, err := getConfiguredCatalogs()

	// Should return error for corrupted JSON
	require.Error(t, err)
	assert.Empty(t, catalogFiles)
}

func TestCatalogPrecedenceOrder(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Create test catalogs with overlapping server name and conflicting tool names
	setupOverlappingCatalogs(t, tempHome)

	ctx := context.Background()

	// Test precedence order: Docker → Configured → CLI-specified
	additionalCatalogs := []string{filepath.Join(tempHome, "cli-catalog.yaml")}
	catalog, err := GetWithOptions(ctx, true, additionalCatalogs)
	require.NoError(t, err)

	// Should contain the server from CLI catalog (highest precedence)
	server, exists := catalog.Servers["overlapping-server"]
	require.True(t, exists, "should contain overlapping server")
	assert.Equal(t, "CLI Catalog Server", server.Tools[0].Description, "should use CLI catalog version (highest precedence)")

	// Test that server also includes other non-overlapping servers
	assert.Contains(t, catalog.Servers, "docker-server", "should contain Docker catalog server")
	assert.Contains(t, catalog.Servers, "custom-server", "should contain configured catalog server")

	// Test that server-level precedence means the entire server (including all tools) is replaced
	// This is correct because during gateway startup, tools from all servers get flattened into one list
	// So we need server-level precedence to prevent incompatible tool sets from mixing
	overlappingServer := catalog.Servers["overlapping-server"]
	require.Len(t, overlappingServer.Tools, 1, "should have exactly the tools from winning server")
	assert.Equal(t, "overlapping-tool", overlappingServer.Tools[0].Name, "tool name should be preserved")
	assert.Equal(t, "CLI Catalog Server", overlappingServer.Tools[0].Description, "should use entire server from highest precedence catalog")
}

// Helper functions

func setupTestCatalogs(t *testing.T, homeDir string) {
	t.Helper()

	// Create .docker/mcp directory structure
	mcpDir := filepath.Join(homeDir, ".docker", "mcp")
	catalogsDir := filepath.Join(mcpDir, "catalogs")
	err := os.MkdirAll(catalogsDir, 0o755)
	require.NoError(t, err)

	// Create catalog.json registry
	catalogRegistry := `{
  "catalogs": {
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
	dockerCatalog := `registry:
  docker-server:
    image: docker/test-server
    tools:
      - name: docker-tool
        description: "Docker Catalog Server"
        container:
          image: docker/test-server
          command: []`
	err = os.WriteFile(filepath.Join(catalogsDir, "docker-mcp.yaml"), []byte(dockerCatalog), 0o644)
	require.NoError(t, err)

	// Create my-catalog.yaml (configured catalog)
	customCatalog := `registry:
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

func setupOverlappingCatalogs(t *testing.T, homeDir string) {
	t.Helper()

	// Create basic structure first
	setupTestCatalogs(t, homeDir)

	mcpDir := filepath.Join(homeDir, ".docker", "mcp")
	catalogsDir := filepath.Join(mcpDir, "catalogs")

	// Update docker-mcp.yaml with overlapping server
	dockerCatalog := `registry:
  docker-server:
    image: docker/test-server
    tools:
      - name: docker-tool
        description: "Docker Catalog Server"
        container:
          image: docker/test-server
          command: []
  overlapping-server:
    image: docker/overlapping-server
    tools:
      - name: overlapping-tool
        description: "Docker Catalog Server"
        container:
          image: docker/overlapping-server
          command: []`
	err := os.WriteFile(filepath.Join(catalogsDir, "docker-mcp.yaml"), []byte(dockerCatalog), 0o644)
	require.NoError(t, err)

	// Update my-catalog.yaml with overlapping server
	customCatalog := `registry:
  custom-server:
    image: custom/test-server
    tools:
      - name: custom-tool
        description: "Custom Catalog Server"
        container:
          image: custom/test-server
          command: []
  overlapping-server:
    image: custom/overlapping-server
    tools:
      - name: overlapping-tool
        description: "Configured Catalog Server"
        container:
          image: custom/overlapping-server
          command: []`
	err = os.WriteFile(filepath.Join(catalogsDir, "my-catalog.yaml"), []byte(customCatalog), 0o644)
	require.NoError(t, err)

	// Create CLI catalog with overlapping server (highest precedence)
	cliCatalog := `registry:
  overlapping-server:
    image: cli/overlapping-server
    tools:
      - name: overlapping-tool
        description: "CLI Catalog Server"
        container:
          image: cli/overlapping-server
          command: []`
	err = os.WriteFile(filepath.Join(homeDir, "cli-catalog.yaml"), []byte(cliCatalog), 0o644)
	require.NoError(t, err)
}
