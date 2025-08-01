package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportCatalogCommand(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Create test catalog registry and configured catalog
	setupTestCatalogRegistry(t, tempHome)

	ctx := context.Background()

	// Test exporting a configured catalog
	outputFile := filepath.Join(tempHome, "exported-catalog.yaml")

	// Create and execute export command
	cmd := exportCatalogCommand()
	cmd.SetArgs([]string{"my-catalog", outputFile})
	cmd.SetContext(ctx)

	err := cmd.Execute()
	require.NoError(t, err, "export command should succeed")

	// Verify the exported file exists and has correct content
	assert.FileExists(t, outputFile, "exported catalog file should exist")

	// Read and verify the exported content
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	exportedContent := string(content)
	assert.Contains(t, exportedContent, "custom-server", "exported catalog should contain custom server")
	assert.Contains(t, exportedContent, "custom/test-server", "exported catalog should contain custom server image")
}

func TestExportDockerCatalogShouldFail(t *testing.T) {
	ctx := context.Background()
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Create test catalog registry
	setupTestCatalogRegistry(t, tempHome)

	// Test exporting Docker catalog should fail
	outputFile := filepath.Join(tempHome, "docker-catalog.yaml")

	cmd := exportCatalogCommand()
	cmd.SetArgs([]string{"docker-mcp", outputFile})
	cmd.SetContext(ctx)

	err := cmd.Execute()
	require.Error(t, err, "exporting Docker catalog should fail")
	assert.Contains(t, err.Error(), "cannot export", "error should mention export restriction")
}

func TestExportNonExistentCatalog(t *testing.T) {
	ctx := context.Background()
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Test exporting non-existent catalog
	outputFile := filepath.Join(tempHome, "nonexistent.yaml")

	cmd := exportCatalogCommand()
	cmd.SetArgs([]string{"nonexistent-catalog", outputFile})
	cmd.SetContext(ctx)

	err := cmd.Execute()
	require.Error(t, err, "exporting non-existent catalog should fail")
	assert.Contains(t, err.Error(), "not found", "error should mention catalog not found")
}

func TestExportInvalidOutputPath(t *testing.T) {
	ctx := context.Background()
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Create test catalog registry
	setupTestCatalogRegistry(t, tempHome)

	// Test exporting to a path where we can't write (use a file as directory path)
	// Create a file that will conflict with the directory creation
	conflictFile := filepath.Join(tempHome, "conflict-file")
	err := os.WriteFile(conflictFile, []byte("test"), 0o644)
	require.NoError(t, err)

	// Try to export to a path that treats the file as a directory
	outputFile := filepath.Join(conflictFile, "catalog.yaml")

	cmd := exportCatalogCommand()
	cmd.SetArgs([]string{"my-catalog", outputFile})
	cmd.SetContext(ctx)

	err = cmd.Execute()
	require.Error(t, err, "exporting to invalid path should fail")
	assert.Contains(t, err.Error(), "not a directory", "error should indicate directory issue")
}

// Helper function to set up test catalog registry
func setupTestCatalogRegistry(t *testing.T, homeDir string) {
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
