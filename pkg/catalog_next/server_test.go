package catalognext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goccy/go-yaml"
	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/docker/mcp-gateway/test/mocks"
)

func TestInspectServer(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with servers whose snapshot catalog.Server has the
	// image field set (matching real TransformToDocker output).
	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "my-server",
							Description: "My test server",
							Image:       "docker/server1:v1",
						},
					},
				},
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server2:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "another-server",
							Description: "Another test server",
							Image:       "docker/server2:v1",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	t.Run("JSON format", func(t *testing.T) {
		output := captureStdout(t, func() {
			err := InspectServer(ctx, dao, nil, catalogObj.Ref, "my-server", workingset.OutputFormatJSON)
			require.NoError(t, err)
		})

		var server InspectResult
		err := json.Unmarshal([]byte(output), &server)
		require.NoError(t, err)
		assert.Equal(t, "my-server", server.Snapshot.Server.Name)
		assert.Equal(t, "docker/server1:v1", server.Image)
		// With no ReadmeURL, a synthesized overview is built from metadata.
		// The description is NOT duplicated (it's already in the UI header);
		// instead we get connection info from the image field.
		assert.Contains(t, server.ReadmeContent, "Runs in Docker container")
		assert.Contains(t, server.ReadmeContent, "docker/server1:v1")
	})

	t.Run("YAML format", func(t *testing.T) {
		output := captureStdout(t, func() {
			err := InspectServer(ctx, dao, nil, catalogObj.Ref, "my-server", workingset.OutputFormatYAML)
			require.NoError(t, err)
		})

		var server InspectResult
		err := yaml.Unmarshal([]byte(output), &server)
		require.NoError(t, err)
		assert.Equal(t, "my-server", server.Snapshot.Server.Name)
		assert.Equal(t, "docker/server1:v1", server.Image)
		assert.Contains(t, server.ReadmeContent, "Runs in Docker container")
	})

	t.Run("HumanReadable format (uses YAML)", func(t *testing.T) {
		output := captureStdout(t, func() {
			err := InspectServer(ctx, dao, nil, catalogObj.Ref, "my-server", workingset.OutputFormatHumanReadable)
			require.NoError(t, err)
		})

		var server InspectResult
		err := yaml.Unmarshal([]byte(output), &server)
		require.NoError(t, err)
		assert.Equal(t, "my-server", server.Snapshot.Server.Name)
		assert.Contains(t, server.ReadmeContent, "Runs in Docker container")
	})
}

func TestInspectServerWithReadme(t *testing.T) {
	readmeContent := "# Notion Remote\n\nThis is a remote server"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/readme.md":
			w.Header().Set("Content-Type", "text/markdown")
			_, _ = w.Write([]byte(readmeContent))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(func() { server.Close() })

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with servers
	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "my-server",
							Description: "My test server",
							ReadmeURL:   server.URL + "/readme.md",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := InspectServer(ctx, dao, nil, catalogObj.Ref, "my-server", workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var inspectResult InspectResult
	err = json.Unmarshal([]byte(output), &inspectResult)
	require.NoError(t, err)
	assert.Equal(t, "my-server", inspectResult.Snapshot.Server.Name)
	assert.Equal(t, "docker/server1:v1", inspectResult.Image)
	assert.Equal(t, readmeContent, inspectResult.ReadmeContent)
}

func TestInspectServerReadmeFetchFailsFallsBackToSynthesized(t *testing.T) {
	// When a ReadmeURL is set but the fetch fails (e.g. private repo, 404),
	// the inspect should not fail. It should fall back to a synthesized overview.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(func() { server.Close() })

	dao := setupTestDB(t)
	ctx := t.Context()

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "my-server",
							Description: "A community MCP server",
							Image:       "docker/server1:v1",
							ReadmeURL:   server.URL + "/nonexistent-readme.md",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := InspectServer(ctx, dao, nil, catalogObj.Ref, "my-server", workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var inspectResult InspectResult
	err = json.Unmarshal([]byte(output), &inspectResult)
	require.NoError(t, err)
	assert.Equal(t, "my-server", inspectResult.Snapshot.Server.Name)
	// The README fetch failed, so it should fall back to a synthesized overview.
	assert.Contains(t, inspectResult.ReadmeContent, "Runs in Docker container")
}

func TestInspectServerNoReadmeURLUsesSynthesized(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "community-server",
							Description: "Short description for overview",
							Image:       "docker/server1:v1",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := InspectServer(ctx, dao, nil, catalogObj.Ref, "community-server", workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var inspectResult InspectResult
	err = json.Unmarshal([]byte(output), &inspectResult)
	require.NoError(t, err)
	assert.Equal(t, "community-server", inspectResult.Snapshot.Server.Name)
	// Synthesized overview includes the image connection info, not the description
	assert.Contains(t, inspectResult.ReadmeContent, "Runs in Docker container")
	assert.NotContains(t, inspectResult.ReadmeContent, "Short description for overview")
}

func TestInspectServerNoReadmeURLNoDescription(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name: "bare-server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := InspectServer(ctx, dao, nil, catalogObj.Ref, "bare-server", workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var inspectResult InspectResult
	err = json.Unmarshal([]byte(output), &inspectResult)
	require.NoError(t, err)
	assert.Equal(t, "bare-server", inspectResult.Snapshot.Server.Name)
	// No ReadmeURL and no Description means empty ReadmeContent
	assert.Empty(t, inspectResult.ReadmeContent)
}

func TestBuildSynthesizedOverview(t *testing.T) {
	t.Run("description only used as last resort", func(t *testing.T) {
		// When there's nothing else to show, fall back to description
		s := &catalog.Server{
			Description: "A simple server",
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Equal(t, "A simple server\n", result)
	})

	t.Run("description omitted when other content exists", func(t *testing.T) {
		s := &catalog.Server{
			Description: "Should not appear",
			Image:       "docker/my-server:latest",
		}
		result := buildSynthesizedOverview(s, nil)
		assert.NotContains(t, result, "Should not appear")
		assert.Contains(t, result, "Runs in Docker container")
		assert.Contains(t, result, "docker/my-server:latest")
	})

	t.Run("remote server connection info", func(t *testing.T) {
		s := &catalog.Server{
			Remote: catalog.Remote{
				URL:       "https://api.example.com/mcp",
				Transport: "streamable-http",
			},
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Contains(t, result, "**Remote MCP server** (streamable-http)")
		assert.Contains(t, result, "Endpoint: `https://api.example.com/mcp`")
	})

	t.Run("docker container connection info", func(t *testing.T) {
		s := &catalog.Server{
			Image: "mcp/postgres:latest",
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Contains(t, result, "**Runs in Docker container** `mcp/postgres:latest`")
	})

	t.Run("with tools", func(t *testing.T) {
		s := &catalog.Server{
			Image: "docker/server:v1",
			Tools: []catalog.Tool{
				{Name: "read_file", Description: "Read a file from disk"},
				{Name: "write_file", Description: "Write a file to disk"},
				{Name: "no_desc"},
			},
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Contains(t, result, "## Tools")
		assert.Contains(t, result, "| read_file | Read a file from disk |")
		assert.Contains(t, result, "| write_file | Write a file to disk |")
		assert.Contains(t, result, "| no_desc | - |")
	})

	t.Run("with secrets shows env var name", func(t *testing.T) {
		s := &catalog.Server{
			Image: "docker/server:v1",
			Secrets: []catalog.Secret{
				{Name: "my-server.api_key", Env: "API_KEY"},
				{Name: "my-server.api_secret", Env: "API_SECRET"},
			},
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Contains(t, result, "## Authentication")
		assert.Contains(t, result, "`API_KEY`")
		assert.Contains(t, result, "`API_SECRET`")
		// Should NOT show the fully qualified internal name
		assert.NotContains(t, result, "my-server.api_key")
	})

	t.Run("with secrets falls back to name when env empty", func(t *testing.T) {
		s := &catalog.Server{
			Image: "docker/server:v1",
			Secrets: []catalog.Secret{
				{Name: "SOME_SECRET"},
			},
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Contains(t, result, "`SOME_SECRET`")
	})

	t.Run("with config schema", func(t *testing.T) {
		s := &catalog.Server{
			Image: "docker/server:v1",
			Config: []any{
				map[string]any{
					"name": "my-server",
					"properties": map[string]any{
						"api_url": map[string]any{
							"type":        "string",
							"description": "The API endpoint URL",
						},
						"timeout": map[string]any{
							"type": "number",
						},
					},
				},
			},
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Contains(t, result, "## Configuration")
		assert.Contains(t, result, "`api_url`: The API endpoint URL")
		assert.Contains(t, result, "`timeout`")
	})

	t.Run("with metadata details", func(t *testing.T) {
		s := &catalog.Server{
			Image: "docker/server:v1",
			Metadata: &catalog.Metadata{
				Category: "Developer Tools",
				License:  "MIT",
				Tags:     []string{"git", "code", "productivity"},
			},
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Contains(t, result, "## Details")
		assert.Contains(t, result, "**Category:** Developer Tools")
		assert.Contains(t, result, "**License:** MIT")
		assert.Contains(t, result, "**Tags:** git, code, productivity")
	})

	t.Run("with registry link", func(t *testing.T) {
		s := &catalog.Server{
			Image: "docker/server:v1",
			Metadata: &catalog.Metadata{
				RegistryURL: "https://registry.modelcontextprotocol.io/servers/my-server",
			},
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Contains(t, result, "## Links")
		assert.Contains(t, result, "[MCP Registry](https://registry.modelcontextprotocol.io/servers/my-server)")
	})

	t.Run("with source repository link from ReadmeURL", func(t *testing.T) {
		s := &catalog.Server{
			Image:     "docker/server:v1",
			ReadmeURL: "https://raw.githubusercontent.com/owner/repo/HEAD/README.md",
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Contains(t, result, "[Source Repository](https://github.com/owner/repo)")
	})

	t.Run("nil server", func(t *testing.T) {
		result := buildSynthesizedOverview(nil, nil)
		assert.Empty(t, result)
	})

	t.Run("empty server", func(t *testing.T) {
		s := &catalog.Server{}
		result := buildSynthesizedOverview(s, nil)
		assert.Empty(t, result)
	})

	t.Run("full metadata community server", func(t *testing.T) {
		s := &catalog.Server{
			Description: "An MCP server for Notion",
			ReadmeURL:   "https://raw.githubusercontent.com/smithery-ai/mcp-servers/HEAD/notion/README.md",
			Remote: catalog.Remote{
				URL:       "https://server.smithery.ai/@smithery/notion/mcp",
				Transport: "streamable-http",
			},
			Secrets: []catalog.Secret{
				{Name: "ai-smithery-smithery-notion.smithery_api_key", Env: "SMITHERY_API_KEY"},
			},
			Metadata: &catalog.Metadata{
				RegistryURL: "https://registry.modelcontextprotocol.io/v0/servers/ai.smithery%2Fsmithery-notion/versions/1.0.0",
			},
		}
		result := buildSynthesizedOverview(s, nil)
		// Description should NOT be in the overview (avoids header duplication)
		assert.NotContains(t, result, "An MCP server for Notion")
		// Connection info
		assert.Contains(t, result, "**Remote MCP server** (streamable-http)")
		// Authentication uses env var names
		assert.Contains(t, result, "`SMITHERY_API_KEY`")
		assert.NotContains(t, result, "ai-smithery-smithery-notion.smithery_api_key")
		// Links
		assert.Contains(t, result, "[MCP Registry]")
		assert.Contains(t, result, "Endpoint: `https://server.smithery.ai/@smithery/notion/mcp`")
		assert.Contains(t, result, "[Source Repository](https://github.com/smithery-ai/mcp-servers)")
	})

	t.Run("with registry response title and status", func(t *testing.T) {
		s := &catalog.Server{
			Image: "docker/server:v1",
		}
		resp := &v0.ServerResponse{
			Server: v0.ServerJSON{
				Title: "aTars MCP",
			},
			Meta: v0.ResponseMeta{
				Official: &v0.RegistryExtensions{
					Status: model.StatusActive,
				},
			},
		}
		result := buildSynthesizedOverview(s, resp)
		assert.Contains(t, result, "# aTars MCP")
		assert.Contains(t, result, "**Status:** active")
	})

	t.Run("with registry response website in links", func(t *testing.T) {
		s := &catalog.Server{
			Image: "docker/server:v1",
		}
		resp := &v0.ServerResponse{
			Server: v0.ServerJSON{
				WebsiteURL: "https://mcp.aarna.ai/mcp",
			},
		}
		result := buildSynthesizedOverview(s, resp)
		assert.Contains(t, result, "[Website](https://mcp.aarna.ai/mcp)")
	})

	t.Run("registry title overrides catalog title", func(t *testing.T) {
		s := &catalog.Server{
			Title: "Old Title",
			Image: "docker/server:v1",
		}
		resp := &v0.ServerResponse{
			Server: v0.ServerJSON{
				Title: "Fresh Title from Registry",
			},
		}
		result := buildSynthesizedOverview(s, resp)
		assert.Contains(t, result, "# Fresh Title from Registry")
		assert.NotContains(t, result, "Old Title")
	})

	t.Run("catalog title used when no registry response", func(t *testing.T) {
		s := &catalog.Server{
			Title: "Catalog Title",
			Image: "docker/server:v1",
		}
		result := buildSynthesizedOverview(s, nil)
		assert.Contains(t, result, "# Catalog Title")
	})

	t.Run("registry response with no official meta omits status", func(t *testing.T) {
		s := &catalog.Server{
			Image: "docker/server:v1",
		}
		resp := &v0.ServerResponse{
			Server: v0.ServerJSON{
				Title: "My Server",
			},
		}
		result := buildSynthesizedOverview(s, resp)
		assert.NotContains(t, result, "Status")
	})

	t.Run("full community server with registry response", func(t *testing.T) {
		s := &catalog.Server{
			Remote: catalog.Remote{
				URL:       "https://mcp.aarna.ai/mcp",
				Transport: "streamable-http",
			},
			Metadata: &catalog.Metadata{
				RegistryURL: "https://registry.modelcontextprotocol.io/v0/servers/ai.aarna%2Fatars-mcp/versions/0.1.0",
			},
		}
		resp := &v0.ServerResponse{
			Server: v0.ServerJSON{
				Title:      "aTars MCP",
				WebsiteURL: "https://mcp.aarna.ai/mcp",
			},
			Meta: v0.ResponseMeta{
				Official: &v0.RegistryExtensions{
					Status: model.StatusActive,
				},
			},
		}
		result := buildSynthesizedOverview(s, resp)
		assert.Contains(t, result, "# aTars MCP")
		assert.Contains(t, result, "**Status:** active")
		assert.Contains(t, result, "[Website](https://mcp.aarna.ai/mcp)")
		assert.Contains(t, result, "[MCP Registry]")
		assert.Contains(t, result, "**Remote MCP server** (streamable-http)")
	})
}

func TestSourceRepoFromReadmeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard github readme URL",
			input:    "https://raw.githubusercontent.com/owner/repo/HEAD/README.md",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "github readme URL with subfolder",
			input:    "https://raw.githubusercontent.com/smithery-ai/mcp-servers/HEAD/notion/README.md",
			expected: "https://github.com/smithery-ai/mcp-servers",
		},
		{
			name:     "non-github URL returned as-is",
			input:    "https://example.com/readme.md",
			expected: "https://example.com/readme.md",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sourceRepoFromReadmeURL(tt.input))
		})
	}
}

// mockRegistryClient implements registryapi.Client for testing.
type mockRegistryClient struct {
	getServerFunc func(ctx context.Context, url *registryapi.ServerURL) (v0.ServerResponse, error)
}

func (m *mockRegistryClient) GetServer(ctx context.Context, url *registryapi.ServerURL) (v0.ServerResponse, error) {
	if m.getServerFunc != nil {
		return m.getServerFunc(ctx, url)
	}
	return v0.ServerResponse{}, fmt.Errorf("not implemented")
}

func (m *mockRegistryClient) GetServerVersions(_ context.Context, _ *registryapi.ServerURL) (v0.ServerListResponse, error) {
	return v0.ServerListResponse{}, nil
}

func (m *mockRegistryClient) ListServers(_ context.Context, _ string, _ string) ([]v0.ServerResponse, error) {
	return nil, nil
}

func TestFetchReadmeViaRegistryAPI(t *testing.T) {
	t.Run("nil metadata", func(t *testing.T) {
		s := &catalog.Server{}
		content, resp := fetchReadmeViaRegistryAPI(t.Context(), &mockRegistryClient{}, s)
		assert.Empty(t, content)
		assert.Nil(t, resp)
	})

	t.Run("empty registry URL", func(t *testing.T) {
		s := &catalog.Server{
			Metadata: &catalog.Metadata{RegistryURL: ""},
		}
		content, resp := fetchReadmeViaRegistryAPI(t.Context(), &mockRegistryClient{}, s)
		assert.Empty(t, content)
		assert.Nil(t, resp)
	})

	t.Run("unparseable registry URL", func(t *testing.T) {
		s := &catalog.Server{
			Metadata: &catalog.Metadata{RegistryURL: ":::not-a-url"},
		}
		content, resp := fetchReadmeViaRegistryAPI(t.Context(), &mockRegistryClient{}, s)
		assert.Empty(t, content)
		assert.Nil(t, resp)
	})

	t.Run("registry API error", func(t *testing.T) {
		client := &mockRegistryClient{
			getServerFunc: func(_ context.Context, _ *registryapi.ServerURL) (v0.ServerResponse, error) {
				return v0.ServerResponse{}, fmt.Errorf("network error")
			},
		}
		s := &catalog.Server{
			Metadata: &catalog.Metadata{
				RegistryURL: "https://registry.modelcontextprotocol.io/v0/servers/test%2Fserver/versions/1.0.0",
			},
		}
		content, resp := fetchReadmeViaRegistryAPI(t.Context(), client, s)
		assert.Empty(t, content)
		assert.Nil(t, resp)
	})

	t.Run("no repository in response", func(t *testing.T) {
		client := &mockRegistryClient{
			getServerFunc: func(_ context.Context, _ *registryapi.ServerURL) (v0.ServerResponse, error) {
				return v0.ServerResponse{
					Server: v0.ServerJSON{
						Name:    "test/server",
						Version: "1.0.0",
					},
				}, nil
			},
		}
		s := &catalog.Server{
			Metadata: &catalog.Metadata{
				RegistryURL: "https://registry.modelcontextprotocol.io/v0/servers/test%2Fserver/versions/1.0.0",
			},
		}
		content, resp := fetchReadmeViaRegistryAPI(t.Context(), client, s)
		assert.Empty(t, content)
		assert.NotNil(t, resp)
	})

	t.Run("non-GitHub repository", func(t *testing.T) {
		client := &mockRegistryClient{
			getServerFunc: func(_ context.Context, _ *registryapi.ServerURL) (v0.ServerResponse, error) {
				return v0.ServerResponse{
					Server: v0.ServerJSON{
						Name:    "test/server",
						Version: "1.0.0",
						Repository: model.Repository{
							URL:    "https://gitlab.com/owner/repo",
							Source: "gitlab",
						},
					},
				}, nil
			},
		}
		s := &catalog.Server{
			Metadata: &catalog.Metadata{
				RegistryURL: "https://registry.modelcontextprotocol.io/v0/servers/test%2Fserver/versions/1.0.0",
			},
		}
		content, resp := fetchReadmeViaRegistryAPI(t.Context(), client, s)
		assert.Empty(t, content)
		assert.NotNil(t, resp)
	})

	t.Run("GitHub repo with fetchable README", func(t *testing.T) {
		// Set up a fake HTTP server to serve the README content
		readmeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("# My Server\n\nThis is the README."))
		}))
		defer readmeServer.Close()

		// We need to test the full chain, but fetch.Untrusted calls the real
		// raw.githubusercontent.com. Instead, test the function with a repo
		// URL that won't resolve. The unit test for BuildGitHubReadmeURL
		// already covers URL construction. Here we verify the plumbing works
		// when the registry API returns a GitHub repo.
		client := &mockRegistryClient{
			getServerFunc: func(_ context.Context, _ *registryapi.ServerURL) (v0.ServerResponse, error) {
				return v0.ServerResponse{
					Server: v0.ServerJSON{
						Name:    "test/server",
						Version: "1.0.0",
						Repository: model.Repository{
							URL:    "https://github.com/test-owner/test-repo",
							Source: "github",
						},
					},
				}, nil
			},
		}
		s := &catalog.Server{
			Metadata: &catalog.Metadata{
				RegistryURL: "https://registry.modelcontextprotocol.io/v0/servers/test%2Fserver/versions/1.0.0",
			},
		}
		// This will attempt to fetch from raw.githubusercontent.com which will
		// likely 404 in tests. That's fine -- the function should return empty.
		content, resp := fetchReadmeViaRegistryAPI(t.Context(), client, s)
		// We can't assert the content because the real fetch will likely fail.
		// This test validates that the function correctly chains through the
		// registry API -> BuildGitHubReadmeURL path without panicking.
		_ = content
		assert.NotNil(t, resp)
	})

	t.Run("GitHub repo with subfolder", func(t *testing.T) {
		client := &mockRegistryClient{
			getServerFunc: func(_ context.Context, _ *registryapi.ServerURL) (v0.ServerResponse, error) {
				return v0.ServerResponse{
					Server: v0.ServerJSON{
						Name:    "test/monorepo-server",
						Version: "1.0.0",
						Repository: model.Repository{
							URL:       "https://github.com/test-owner/monorepo",
							Source:    "github",
							Subfolder: "packages/mcp-server",
						},
					},
				}, nil
			},
		}
		s := &catalog.Server{
			Metadata: &catalog.Metadata{
				RegistryURL: "https://registry.modelcontextprotocol.io/v0/servers/test%2Fmonorepo-server/versions/1.0.0",
			},
		}
		// Same as above -- real fetch will fail but the path should not panic
		content, resp := fetchReadmeViaRegistryAPI(t.Context(), client, s)
		_ = content
		assert.NotNil(t, resp)
	})
}

func TestInspectServerNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with servers
	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name: "my-server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	err = InspectServer(ctx, dao, nil, catalogObj.Ref, "nonexistent-server", workingset.OutputFormatJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server nonexistent-server not found in catalog test/catalog:latest")
}

func TestInspectServerCatalogNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := InspectServer(ctx, dao, nil, "test/nonexistent:latest", "some-server", workingset.OutputFormatJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get catalog")
}

func TestInspectServerUnsupportedFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with servers
	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name: "my-server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	err = InspectServer(ctx, dao, nil, catalogObj.Ref, "my-server", workingset.OutputFormat("unsupported"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format: unsupported")
}

func TestInspectServerInvalidCatalogRef(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := InspectServer(ctx, dao, nil, ":::invalid-ref", "some-server", workingset.OutputFormatJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse oci-reference")
}

func TestInspectServerDifferentServerTypes(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with different server types
	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/image-server:v1",
					Tools: []string{"tool1", "tool2"},
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "image-server",
							Description: "An image-based server",
						},
					},
				},
				{
					Type:     workingset.ServerTypeRemote,
					Endpoint: "https://example.com/mcp",
					Tools:    []string{"remote-tool"},
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "remote-server",
							Description: "A remote server",
						},
					},
				},
				{
					Type:   workingset.ServerTypeRegistry,
					Source: "https://registry.example.com/server",
					Tools:  []string{"registry-tool"},
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "registry-server",
							Description: "A registry server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	t.Run("inspect image server", func(t *testing.T) {
		output := captureStdout(t, func() {
			err := InspectServer(ctx, dao, nil, catalogObj.Ref, "image-server", workingset.OutputFormatJSON)
			require.NoError(t, err)
		})

		var server Server
		err := json.Unmarshal([]byte(output), &server)
		require.NoError(t, err)
		assert.Equal(t, "image-server", server.Snapshot.Server.Name)
		assert.Equal(t, workingset.ServerTypeImage, server.Type)
		assert.Equal(t, "docker/image-server:v1", server.Image)
		assert.Equal(t, []string{"tool1", "tool2"}, server.Tools)
	})

	t.Run("inspect remote server", func(t *testing.T) {
		output := captureStdout(t, func() {
			err := InspectServer(ctx, dao, nil, catalogObj.Ref, "remote-server", workingset.OutputFormatJSON)
			require.NoError(t, err)
		})

		var server Server
		err := json.Unmarshal([]byte(output), &server)
		require.NoError(t, err)
		assert.Equal(t, "remote-server", server.Snapshot.Server.Name)
		assert.Equal(t, workingset.ServerTypeRemote, server.Type)
		assert.Equal(t, "https://example.com/mcp", server.Endpoint)
		assert.Equal(t, []string{"remote-tool"}, server.Tools)
	})

	t.Run("inspect registry server", func(t *testing.T) {
		output := captureStdout(t, func() {
			err := InspectServer(ctx, dao, nil, catalogObj.Ref, "registry-server", workingset.OutputFormatJSON)
			require.NoError(t, err)
		})

		var server Server
		err := json.Unmarshal([]byte(output), &server)
		require.NoError(t, err)
		assert.Equal(t, "registry-server", server.Snapshot.Server.Name)
		assert.Equal(t, workingset.ServerTypeRegistry, server.Type)
		assert.Equal(t, "https://registry.example.com/server", server.Source)
		assert.Equal(t, []string{"registry-tool"}, server.Tools)
	})
}

func TestListServersNoFilters(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	// Create a catalog with multiple servers
	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "server-one",
							Description: "First server",
						},
					},
				},
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server2:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "server-two",
							Description: "Second server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, catalogObj.Ref, []string{}, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, catalogObj.Ref, result["catalog"])
	assert.Equal(t, catalogObj.Title, result["title"])
	servers := result["servers"].([]any)
	assert.Len(t, servers, 2)
}

func TestListServersFilterByName(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "my-server",
							Description: "My server",
						},
					},
				},
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server2:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "other-server",
							Description: "Other server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, catalogObj.Ref, []string{"name=my"}, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	servers := result["servers"].([]any)
	assert.Len(t, servers, 1)
}

func TestListServersFilterByNameCaseInsensitive(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "MyServer",
							Description: "Test server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, catalogObj.Ref, []string{"name=myserver"}, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	servers := result["servers"].([]any)
	assert.Len(t, servers, 1)
}

func TestListServersFilterByNamePartialMatch(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "my-awesome-server",
							Description: "Test server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, catalogObj.Ref, []string{"name=awesome"}, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	servers := result["servers"].([]any)
	assert.Len(t, servers, 1)
}

func TestListServersFilterNoMatches(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "my-server",
							Description: "Test server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, catalogObj.Ref, []string{"name=nonexistent"}, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	servers := result["servers"].([]any)
	assert.Empty(t, servers)
}

func TestListServersWithoutSnapshot(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:     workingset.ServerTypeImage,
					Image:    "docker/server1:v1",
					Snapshot: nil, // No snapshot
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, catalogObj.Ref, []string{"name=test"}, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Server without snapshot should not match any name filter
	servers := result["servers"].([]any)
	assert.Empty(t, servers)
}

func TestListServersInvalidFilter(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	err = ListServers(ctx, dao, catalogObj.Ref, []string{"invalid"}, workingset.OutputFormatJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filter format")
}

func TestListServersUnsupportedFilterKey(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	err = ListServers(ctx, dao, catalogObj.Ref, []string{"unsupported=value"}, workingset.OutputFormatJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported filter key")
}

func TestListServersCatalogNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	err := ListServers(ctx, dao, "test/nonexistent:latest", []string{}, workingset.OutputFormatJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get catalog")
}

func TestListServersNormalizesCatalogRef(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with a normalized reference (what the db would store)
	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name: "my-server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	// Query with a non-normalized reference (without :latest tag)
	// This should still find the catalog because the code normalizes the ref
	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, "test/catalog", []string{}, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	servers := result["servers"].([]any)
	assert.Len(t, servers, 1)
}

func TestListServersYAMLFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name: "server-one",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, catalogObj.Ref, []string{}, workingset.OutputFormatYAML)
		require.NoError(t, err)
	})

	var result map[string]any
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, catalogObj.Ref, result["catalog"])
	assert.Equal(t, catalogObj.Title, result["title"])
}

func TestListServersHumanReadableFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "server-one",
							Title:       "Server One",
							Description: "First server",
							Tools: []catalog.Tool{
								{Name: "tool1"},
								{Name: "tool2"},
							},
						},
					},
				},
				{
					Type:   workingset.ServerTypeRegistry,
					Source: "https://example.com/api",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name: "server-two",
						},
					},
				},
				{
					Type:     workingset.ServerTypeRemote,
					Endpoint: "https://remote.example.com",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name: "server-three",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, catalogObj.Ref, []string{}, workingset.OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	// Verify human-readable format contains expected elements
	assert.Contains(t, output, "Catalog: "+catalogObj.Ref)
	assert.Contains(t, output, "Title: Test Catalog")
	assert.Contains(t, output, "Servers (3)")
	assert.Contains(t, output, "server-one")
	assert.Contains(t, output, "Title: Server One")
	assert.Contains(t, output, "Description: First server")
	assert.Contains(t, output, "Type: image")
	assert.Contains(t, output, "Image: docker/server1:v1")
	assert.Contains(t, output, "Tools: 2")
	assert.Contains(t, output, "server-two")
	assert.Contains(t, output, "Type: registry")
	assert.Contains(t, output, "Source: https://example.com/api")
	assert.Contains(t, output, "server-three")
	assert.Contains(t, output, "Type: remote")
	assert.Contains(t, output, "Endpoint: https://remote.example.com")
}

func TestListServersHumanReadableNoServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Empty Catalog",
			Servers: []Server{
				{
					Type:     workingset.ServerTypeImage,
					Image:    "docker/server1:v1",
					Snapshot: nil, // No snapshot
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, catalogObj.Ref, []string{"name=nonexistent"}, workingset.OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "No servers found")
}

func TestListServersUnsupportedFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	err = ListServers(ctx, dao, catalogObj.Ref, []string{}, "unsupported")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestListServersServersSortedByName(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server1:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name: "zebra-server",
						},
					},
				},
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server2:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name: "alpha-server",
						},
					},
				},
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/server3:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name: "beta-server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, catalogObj.Ref, []string{}, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	servers := result["servers"].([]any)
	require.Len(t, servers, 3)

	// Verify servers are sorted alphabetically by name
	firstServer := servers[0].(map[string]any)
	snapshot := firstServer["snapshot"].(map[string]any)
	server := snapshot["server"].(map[string]any)
	assert.Equal(t, "alpha-server", server["name"])

	secondServer := servers[1].(map[string]any)
	snapshot = secondServer["snapshot"].(map[string]any)
	server = snapshot["server"].(map[string]any)
	assert.Equal(t, "beta-server", server["name"])

	thirdServer := servers[2].(map[string]any)
	snapshot = thirdServer["snapshot"].(map[string]any)
	server = snapshot["server"].(map[string]any)
	assert.Equal(t, "zebra-server", server["name"])
}

func TestParseFilters(t *testing.T) {
	tests := []struct {
		name        string
		filters     []string
		expected    []serverFilter
		expectError bool
		errorMsg    string
	}{
		{
			name:     "single filter",
			filters:  []string{"name=test"},
			expected: []serverFilter{{key: "name", value: "test"}},
		},
		{
			name:     "multiple filters",
			filters:  []string{"name=test", "type=image"},
			expected: []serverFilter{{key: "name", value: "test"}, {key: "type", value: "image"}},
		},
		{
			name:     "empty filters",
			filters:  []string{},
			expected: []serverFilter{},
		},
		{
			name:        "invalid filter format - no equals",
			filters:     []string{"invalid"},
			expectError: true,
			errorMsg:    "invalid filter format",
		},
		{
			name:        "invalid filter format - multiple equals",
			filters:     []string{"key=value=extra"},
			expected:    []serverFilter{{key: "key", value: "value=extra"}}, // SplitN allows this
			expectError: false,
		},
		{
			name:     "filter with empty value",
			filters:  []string{"name="},
			expected: []serverFilter{{key: "name", value: ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFilters(tt.filters)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestAddServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create an initial catalog
	catalogObj := Catalog{
		Ref: "test/catalog:latest",
		CatalogArtifact: CatalogArtifact{
			Title: "Test Catalog",
			Servers: []Server{
				{
					Type:  workingset.ServerTypeImage,
					Image: "docker/existing-server:v1",
					Snapshot: &workingset.ServerSnapshot{
						Server: catalog.Server{
							Name:        "existing-server",
							Description: "Existing server",
						},
					},
				},
			},
		},
	}

	dbCat, err := catalogObj.ToDb()
	require.NoError(t, err)
	err = dao.UpsertCatalog(ctx, dbCat)
	require.NoError(t, err)

	t.Run("no servers provided", func(t *testing.T) {
		err := AddServers(ctx, dao, mocks.NewMockRegistryAPIClient(), mocks.NewMockOCIService(), catalogObj.Ref, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one server must be specified")
	})

	t.Run("invalid catalog reference", func(t *testing.T) {
		err := AddServers(ctx, dao, mocks.NewMockRegistryAPIClient(), mocks.NewMockOCIService(), ":::invalid", []string{
			"docker/test:latest",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse oci-reference")
	})

	t.Run("catalog not found", func(t *testing.T) {
		err := AddServers(ctx, dao, mocks.NewMockRegistryAPIClient(), mocks.NewMockOCIService(), "test/nonexistent:latest", []string{
			"docker/test:latest",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get catalog")
	})

	t.Run("invalid server reference", func(t *testing.T) {
		err := AddServers(ctx, dao, mocks.NewMockRegistryAPIClient(), mocks.NewMockOCIService(), catalogObj.Ref, []string{
			"invalid://reference",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve server reference")
	})
}

func TestAddServersUpsert(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	mockOci := mocks.NewMockOCIService(mocks.WithLocalImages([]mocks.MockImage{
		{
			Ref: "existing-server:v2",
			Labels: map[string]string{
				"io.docker.server.metadata": "name: existing-server\ntype: server\nimage: existing-server:v2",
			},
			DigestString: "sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		},
	}))

	t.Run("upsert replaces existing", func(t *testing.T) {
		catalogObj := Catalog{
			Ref: "test/upsert-catalog:latest",
			CatalogArtifact: CatalogArtifact{
				Title: "Upsert Catalog",
				Servers: []Server{
					{
						Type:  workingset.ServerTypeImage,
						Image: "existing-server:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name:  "existing-server",
								Type:  "server",
								Image: "existing-server:v1",
							},
						},
					},
				},
			},
		}

		dbCat, err := catalogObj.ToDb()
		require.NoError(t, err)
		err = dao.UpsertCatalog(ctx, dbCat)
		require.NoError(t, err)

		// Add a server with the same name but different image -- should upsert
		err = AddServers(ctx, dao, mocks.NewMockRegistryAPIClient(), mockOci, catalogObj.Ref, []string{
			"docker://existing-server:v2",
		})
		require.NoError(t, err)

		dbCat2, err := dao.GetCatalog(ctx, catalogObj.Ref)
		require.NoError(t, err)
		cat := NewFromDb(dbCat2)
		assert.Len(t, cat.Servers, 1)
		assert.Equal(t, "existing-server", cat.Servers[0].Snapshot.Server.Name)
		assert.Equal(t, "existing-server:v2", cat.Servers[0].Image)
	})

	t.Run("upsert preserves other servers", func(t *testing.T) {
		catalogObj := Catalog{
			Ref: "test/upsert-catalog2:latest",
			CatalogArtifact: CatalogArtifact{
				Title: "Upsert Catalog 2",
				Servers: []Server{
					{
						Type:  workingset.ServerTypeImage,
						Image: "existing-server:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name:  "existing-server",
								Type:  "server",
								Image: "existing-server:v1",
							},
						},
					},
					{
						Type:  workingset.ServerTypeImage,
						Image: "other-server:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name:  "other-server",
								Type:  "server",
								Image: "other-server:v1",
							},
						},
					},
				},
			},
		}

		dbCat, err := catalogObj.ToDb()
		require.NoError(t, err)
		err = dao.UpsertCatalog(ctx, dbCat)
		require.NoError(t, err)

		// Upsert only existing-server
		err = AddServers(ctx, dao, mocks.NewMockRegistryAPIClient(), mockOci, catalogObj.Ref, []string{
			"docker://existing-server:v2",
		})
		require.NoError(t, err)

		dbCat2, err := dao.GetCatalog(ctx, catalogObj.Ref)
		require.NoError(t, err)
		cat := NewFromDb(dbCat2)
		assert.Len(t, cat.Servers, 2)
		// other-server should be first (preserved in place)
		assert.Equal(t, "other-server", cat.Servers[0].Snapshot.Server.Name)
		assert.Equal(t, "other-server:v1", cat.Servers[0].Image)
		// existing-server should be at the end (re-added after filtering)
		assert.Equal(t, "existing-server", cat.Servers[1].Snapshot.Server.Name)
		assert.Equal(t, "existing-server:v2", cat.Servers[1].Image)
	})
}

func TestRemoveServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	t.Run("remove single server", func(t *testing.T) {
		// Create a catalog with multiple servers
		catalogObj := Catalog{
			Ref: "test/catalog:latest",
			CatalogArtifact: CatalogArtifact{
				Title: "Test Catalog",
				Servers: []Server{
					{
						Type:  workingset.ServerTypeImage,
						Image: "docker/server1:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name: "server-one",
							},
						},
					},
					{
						Type:  workingset.ServerTypeImage,
						Image: "docker/server2:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name: "server-two",
							},
						},
					},
				},
			},
		}

		dbCat, err := catalogObj.ToDb()
		require.NoError(t, err)
		err = dao.UpsertCatalog(ctx, dbCat)
		require.NoError(t, err)

		output := captureStdout(t, func() {
			err := RemoveServers(ctx, dao, catalogObj.Ref, []string{"server-one"})
			require.NoError(t, err)
		})

		assert.Contains(t, output, "Removed 1 server(s)")

		// Verify server was removed
		dbCat2, err := dao.GetCatalog(ctx, catalogObj.Ref)
		require.NoError(t, err)
		cat := NewFromDb(dbCat2)
		assert.Len(t, cat.Servers, 1)
		assert.Nil(t, cat.FindServer("server-one"))
		assert.NotNil(t, cat.FindServer("server-two"))
	})

	t.Run("remove multiple servers", func(t *testing.T) {
		// Create a catalog with multiple servers
		catalogObj := Catalog{
			Ref: "test/catalog2:latest",
			CatalogArtifact: CatalogArtifact{
				Title: "Test Catalog",
				Servers: []Server{
					{
						Type:  workingset.ServerTypeImage,
						Image: "docker/server1:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name: "server-one",
							},
						},
					},
					{
						Type:  workingset.ServerTypeImage,
						Image: "docker/server2:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name: "server-two",
							},
						},
					},
					{
						Type:  workingset.ServerTypeImage,
						Image: "docker/server3:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name: "server-three",
							},
						},
					},
				},
			},
		}

		dbCat, err := catalogObj.ToDb()
		require.NoError(t, err)
		err = dao.UpsertCatalog(ctx, dbCat)
		require.NoError(t, err)

		output := captureStdout(t, func() {
			err := RemoveServers(ctx, dao, catalogObj.Ref, []string{"server-one", "server-three"})
			require.NoError(t, err)
		})

		assert.Contains(t, output, "Removed 2 server(s)")

		// Verify servers were removed
		dbCat3, err := dao.GetCatalog(ctx, catalogObj.Ref)
		require.NoError(t, err)
		cat := NewFromDb(dbCat3)
		assert.Len(t, cat.Servers, 1)
		assert.Nil(t, cat.FindServer("server-one"))
		assert.NotNil(t, cat.FindServer("server-two"))
		assert.Nil(t, cat.FindServer("server-three"))
	})

	t.Run("remove nonexistent server", func(t *testing.T) {
		// Create a catalog with servers
		catalogObj := Catalog{
			Ref: "test/catalog3:latest",
			CatalogArtifact: CatalogArtifact{
				Title: "Test Catalog",
				Servers: []Server{
					{
						Type:  workingset.ServerTypeImage,
						Image: "docker/server1:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name: "server-one",
							},
						},
					},
				},
			},
		}

		dbCat, err := catalogObj.ToDb()
		require.NoError(t, err)
		err = dao.UpsertCatalog(ctx, dbCat)
		require.NoError(t, err)

		err = RemoveServers(ctx, dao, catalogObj.Ref, []string{"nonexistent-server"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no matching servers found to remove")
	})

	t.Run("remove server without snapshot", func(t *testing.T) {
		// Create a catalog with server without snapshot
		catalogObj := Catalog{
			Ref: "test/catalog4:latest",
			CatalogArtifact: CatalogArtifact{
				Title: "Test Catalog",
				Servers: []Server{
					{
						Type:     workingset.ServerTypeImage,
						Image:    "docker/server1:v1",
						Snapshot: nil,
					},
				},
			},
		}

		dbCat, err := catalogObj.ToDb()
		require.NoError(t, err)
		err = dao.UpsertCatalog(ctx, dbCat)
		require.NoError(t, err)

		err = RemoveServers(ctx, dao, catalogObj.Ref, []string{"some-name"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no matching servers found to remove")

		// Verify server without snapshot is still there
		dbCat4, err := dao.GetCatalog(ctx, catalogObj.Ref)
		require.NoError(t, err)
		cat := NewFromDb(dbCat4)
		assert.Len(t, cat.Servers, 1)
	})

	t.Run("no server names provided", func(t *testing.T) {
		err := RemoveServers(ctx, dao, "test/catalog:latest", []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one server name must be specified")
	})

	t.Run("invalid catalog reference", func(t *testing.T) {
		err := RemoveServers(ctx, dao, ":::invalid", []string{"server-one"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse oci-reference")
	})

	t.Run("catalog not found", func(t *testing.T) {
		err := RemoveServers(ctx, dao, "test/nonexistent:latest", []string{"server-one"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get catalog")
	})

	t.Run("remove all servers from catalog", func(t *testing.T) {
		// Create a catalog with one server
		catalogObj := Catalog{
			Ref: "test/catalog5:latest",
			CatalogArtifact: CatalogArtifact{
				Title: "Test Catalog",
				Servers: []Server{
					{
						Type:  workingset.ServerTypeImage,
						Image: "docker/server1:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name: "only-server",
							},
						},
					},
				},
			},
		}

		dbCat, err := catalogObj.ToDb()
		require.NoError(t, err)
		err = dao.UpsertCatalog(ctx, dbCat)
		require.NoError(t, err)

		output := captureStdout(t, func() {
			err := RemoveServers(ctx, dao, catalogObj.Ref, []string{"only-server"})
			require.NoError(t, err)
		})

		assert.Contains(t, output, "Removed 1 server(s)")

		// Verify catalog is now empty
		dbCat5, err := dao.GetCatalog(ctx, catalogObj.Ref)
		require.NoError(t, err)
		cat := NewFromDb(dbCat5)
		assert.Empty(t, cat.Servers)
	})
}
