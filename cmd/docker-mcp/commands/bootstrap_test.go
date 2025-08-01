package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
)

func TestBootstrapCatalogCommand(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Setup Docker catalog for testing
	setupTestDockerCatalog(t, tempHome)

	ctx := context.Background()

	// Test bootstrapping a catalog
	outputFile := filepath.Join(tempHome, "bootstrap-catalog.yaml")

	// Create and execute bootstrap command
	cmd := bootstrapCatalogCommand()
	cmd.SetArgs([]string{outputFile})
	cmd.SetContext(ctx)

	err := cmd.Execute()
	require.NoError(t, err, "bootstrap command should succeed")

	// Verify the bootstrap file exists and has correct content
	assert.FileExists(t, outputFile, "bootstrap catalog file should exist")

	// Read and verify the bootstrap content
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	bootstrapContent := string(content)
	assert.Contains(t, bootstrapContent, "registry:", "bootstrap catalog should have registry section")
	assert.Contains(t, bootstrapContent, catalog.DockerHubServerName, "bootstrap catalog should contain dockerhub server")
	assert.Contains(t, bootstrapContent, catalog.DockerCLIServerName, "bootstrap catalog should contain docker server")

	// Parse the YAML to ensure it's valid
	var registry map[string]any
	err = yaml.Unmarshal(content, &registry)
	require.NoError(t, err, "bootstrap catalog should be valid YAML")

	// Verify structure
	registryMap, ok := registry["registry"].(map[string]any)
	require.True(t, ok, "bootstrap catalog should have registry map")

	// Check that we have exactly the Docker servers
	assert.Len(t, registryMap, 2, "bootstrap catalog should contain exactly 2 servers")
	assert.Contains(t, registryMap, catalog.DockerHubServerName, "should contain dockerhub server")
	assert.Contains(t, registryMap, catalog.DockerCLIServerName, "should contain docker server")
}

func TestBootstrapExistingFile(t *testing.T) {
	ctx := context.Background()
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Setup Docker catalog for testing
	setupTestDockerCatalog(t, tempHome)

	// Create an existing file
	outputFile := filepath.Join(tempHome, "existing-catalog.yaml")
	err := os.WriteFile(outputFile, []byte("existing content"), 0o644)
	require.NoError(t, err)

	// Test bootstrapping over existing file should fail
	cmd := bootstrapCatalogCommand()
	cmd.SetArgs([]string{outputFile})
	cmd.SetContext(ctx)

	err = cmd.Execute()
	require.Error(t, err, "bootstrapping over existing file should fail")
	assert.Contains(t, err.Error(), "already exists", "error should mention file already exists")

	// Verify original content is preserved
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "existing content", string(content), "original file content should be preserved")
}

func TestBootstrapInvalidPath(t *testing.T) {
	ctx := context.Background()
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Setup Docker catalog for testing
	setupTestDockerCatalog(t, tempHome)

	// Test bootstrapping to invalid path (use a file as directory path)
	// Create a file that will conflict with the directory creation
	conflictFile := filepath.Join(tempHome, "conflict-file")
	err := os.WriteFile(conflictFile, []byte("test"), 0o644)
	require.NoError(t, err)

	// Try to bootstrap to a path that treats the file as a directory
	outputFile := filepath.Join(conflictFile, "catalog.yaml")

	cmd := bootstrapCatalogCommand()
	cmd.SetArgs([]string{outputFile})
	cmd.SetContext(ctx)

	err = cmd.Execute()
	require.Error(t, err, "bootstrapping to invalid path should fail")
	assert.Contains(t, err.Error(), "not a directory", "error should indicate directory issue")
}

func TestBootstrapDockerEntriesExtraction(t *testing.T) {
	// Create temporary home directory
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("HOME")

	// Setup a more detailed Docker catalog for testing
	setupDetailedTestDockerCatalog(t, tempHome)

	ctx := context.Background()
	outputFile := filepath.Join(tempHome, "detailed-bootstrap.yaml")

	// Create and execute bootstrap command
	cmd := bootstrapCatalogCommand()
	cmd.SetArgs([]string{outputFile})
	cmd.SetContext(ctx)

	err := cmd.Execute()
	require.NoError(t, err, "bootstrap command should succeed")

	// Read the bootstrap content
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	bootstrapContent := string(content)

	// Verify detailed Docker Hub server content is preserved
	assert.Contains(t, bootstrapContent, "Docker Hub official MCP server", "should preserve dockerhub description")
	assert.Contains(t, bootstrapContent, "mcp/dockerhub", "should preserve dockerhub image")

	// Verify detailed Docker CLI server content is preserved
	assert.Contains(t, bootstrapContent, "Use the Docker CLI", "should preserve docker description")
	assert.Contains(t, bootstrapContent, "docker@sha256:", "should preserve docker image")

	// Verify other servers are NOT included
	assert.NotContains(t, bootstrapContent, "github", "should not contain other servers like github")
	assert.NotContains(t, bootstrapContent, "elasticsearch", "should not contain other servers like elasticsearch")
}

// Helper function to set up basic test Docker catalog
func setupTestDockerCatalog(t *testing.T, homeDir string) {
	t.Helper()

	// Create .docker/mcp directory structure
	mcpDir := filepath.Join(homeDir, ".docker", "mcp")
	catalogsDir := filepath.Join(mcpDir, "catalogs")
	err := os.MkdirAll(catalogsDir, 0o755)
	require.NoError(t, err)

	// Create catalog.json registry
	catalogRegistry := `{
  "catalogs": {
    "docker-mcp": {
      "displayName": "Docker MCP Catalog",
      "url": "",
      "lastUpdate": "2025-08-01T00:00:00Z"
    }
  }
}`
	err = os.WriteFile(filepath.Join(mcpDir, "catalog.json"), []byte(catalogRegistry), 0o644)
	require.NoError(t, err)

	// Create minimal docker-mcp.yaml catalog with dockerhub and docker servers
	dockerCatalog := `registry:
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
}

// Helper function to set up detailed test Docker catalog with more servers
func setupDetailedTestDockerCatalog(t *testing.T, homeDir string) {
	t.Helper()

	// Create .docker/mcp directory structure
	mcpDir := filepath.Join(homeDir, ".docker", "mcp")
	catalogsDir := filepath.Join(mcpDir, "catalogs")
	err := os.MkdirAll(catalogsDir, 0o755)
	require.NoError(t, err)

	// Create catalog.json registry
	catalogRegistry := `{
  "catalogs": {
    "docker-mcp": {
      "displayName": "Docker MCP Catalog",
      "url": "",
      "lastUpdate": "2025-08-01T00:00:00Z"
    }
  }
}`
	err = os.WriteFile(filepath.Join(mcpDir, "catalog.json"), []byte(catalogRegistry), 0o644)
	require.NoError(t, err)

	// Create more detailed docker-mcp.yaml catalog with multiple servers
	dockerCatalog := `registry:
  dockerhub:
    description: "Docker Hub official MCP server."
    title: "Docker Hub"
    type: "server"
    image: "mcp/dockerhub@sha256:b3a124cc092a2eb24b3cad69d9ea0f157581762d993d599533b1802740b2c262"
    tools:
      - name: "search"
      - name: "getRepositoryInfo"
      - name: "createRepository"
    secrets:
      - name: "dockerhub.pat_token"
        env: "HUB_PAT_TOKEN"
    command:
      - "--transport=stdio"
      - "--username={{dockerhub.username}}"
    config:
      - name: "dockerhub"
        description: "Configure connection to Docker Hub"
        type: "object"
        properties:
          username:
            type: "string"
  docker:
    description: "Use the Docker CLI."
    title: "Docker"
    type: "poci"
    image: "docker@sha256:cf5c79bfb90a1b8ef3947b013fe61b3d66ad790ab4bcf3ee5319e8b88134f553"
    tools:
      - name: "docker"
        description: "use the docker cli"
        parameters:
          type: "object"
          properties:
            args:
              type: "array"
              description: "Arguments to pass to the Docker command"
              items:
                type: "string"
          required:
            - "args"
        container:
          image: "docker@sha256:cf5c79bfb90a1b8ef3947b013fe61b3d66ad790ab4bcf3ee5319e8b88134f553"
          command:
            - "{{args|into}}"
          volumes:
            - "/var/run/docker.sock:/var/run/docker.sock"
  github:
    description: "GitHub integration server."
    title: "GitHub"
    image: "mcp/github@sha256:test789"
    tools:
      - name: "create_issue"
  elasticsearch:
    description: "Elasticsearch search server."
    title: "Elasticsearch"  
    image: "mcp/elasticsearch@sha256:testxyz"
    tools:
      - name: "search"`
	err = os.WriteFile(filepath.Join(catalogsDir, "docker-mcp.yaml"), []byte(dockerCatalog), 0o644)
	require.NoError(t, err)
}
