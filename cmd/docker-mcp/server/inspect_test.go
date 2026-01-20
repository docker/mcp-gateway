package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-gateway/pkg/docker"
)

// TestInspectDynamicToolsServer tests that servers with dynamic tools
// (dynamic: { tools: true }) are handled correctly without attempting to fetch tools
func TestInspectDynamicToolsServer(t *testing.T) {
	ctx, home, dockerClient := setupInspectTest(t)

	// Create mock HTTP server for readme
	readmeContent := "# Notion Remote\n\nThis is a remote server with dynamic tools."

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/readme.md":
			w.Header().Set("Content-Type", "text/markdown")
			_, _ = w.Write([]byte(readmeContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create catalog with a server that has dynamic tools
	catalogYAML := `registry:
  notion-remote:
    description: Remote Notion server with dynamic tools
    title: Notion Remote
    type: remote
    dynamic:
      tools: true
    readme: ` + server.URL + `/readme.md
`
	writeCatalogFile(t, home, catalogYAML)

	// Inspect should succeed without attempting HTTP fetch for tools
	info, err := Inspect(ctx, dockerClient, "notion-remote")
	require.NoError(t, err, "Inspect should succeed for dynamic tools server")

	// Dynamic tools servers should return empty tools array at inspect time
	assert.Empty(t, info.Tools, "Dynamic tools server should return empty tools array")

	// Should have fetched readme
	assert.Equal(t, readmeContent, info.Readme)
}

// TestInspectStaticToolsServer tests that servers with static toolsUrl
// correctly fetch and parse tools
func TestInspectStaticToolsServer(t *testing.T) {
	ctx, home, dockerClient := setupInspectTest(t)

	// Create mock HTTP server for tools and readme
	toolsResponse := []Tool{
		{
			Name:        "test-tool-1",
			Description: "First test tool",
			Enabled:     true,
		},
		{
			Name:        "test-tool-2",
			Description: "Second test tool",
			Enabled:     true,
		},
	}
	toolsJSON, err := json.Marshal(toolsResponse)
	require.NoError(t, err)

	readmeContent := "# Test Server\n\nThis is a test server."

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tools.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(toolsJSON)
		case "/readme.md":
			w.Header().Set("Content-Type", "text/markdown")
			_, _ = w.Write([]byte(readmeContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create catalog with a server that has static toolsUrl
	catalogYAML := `registry:
  notion:
    description: Static Notion server
    title: Notion
    type: server
    readme: ` + server.URL + `/readme.md
    toolsUrl: ` + server.URL + `/tools.json
`
	writeCatalogFile(t, home, catalogYAML)

	// Inspect should successfully fetch and parse tools
	info, err := Inspect(ctx, dockerClient, "notion")
	require.NoError(t, err, "Inspect should succeed for static tools server")

	// Should have fetched tools from the mock server
	assert.Len(t, info.Tools, 2, "Should have 2 tools")
	assert.Equal(t, "test-tool-1", info.Tools[0].Name)
	assert.Equal(t, "test-tool-2", info.Tools[1].Name)

	// Should have fetched readme
	assert.Equal(t, readmeContent, info.Readme)
}

// Test helpers

func setupInspectTest(t *testing.T) (context.Context, string, docker.Client) {
	t.Helper()

	// Create temporary home directory
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create mock Docker client
	dockerClient := &fakeDocker{}

	return t.Context(), home, dockerClient
}

func writeCatalogFile(t *testing.T, home, content string) {
	t.Helper()

	// Write catalog file in the catalogs subdirectory
	catalogsDir := filepath.Join(home, ".docker", "mcp", "catalogs")
	err := os.MkdirAll(catalogsDir, 0o755)
	require.NoError(t, err)

	catalogFile := filepath.Join(catalogsDir, catalog.DockerCatalogFilename)
	err = os.WriteFile(catalogFile, []byte(content), 0o644)
	require.NoError(t, err)

	// Create catalog.json registry file to register the docker-mcp catalog
	catalogRegistry := `{
  "catalogs": {
    "docker-mcp": {
      "displayName": "Docker MCP Default Catalog",
      "url": "docker-mcp.yaml",
      "lastUpdate": "2024-01-01T00:00:00Z"
    }
  }
}`
	mcpDir := filepath.Join(home, ".docker", "mcp")
	catalogRegistryFile := filepath.Join(mcpDir, "catalog.json")
	err = os.WriteFile(catalogRegistryFile, []byte(catalogRegistry), 0o644)
	require.NoError(t, err)

	// Create empty tools.yaml to avoid Docker volume access
	// This prevents the Inspect function from trying to read tool config from Docker
	toolsFile := filepath.Join(mcpDir, "tools.yaml")
	err = os.WriteFile(toolsFile, []byte(""), 0o644)
	require.NoError(t, err)
}
