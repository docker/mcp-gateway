package catalognext

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/docker/mcp-gateway/test/mocks"
)

func TestResolveCatalogRef(t *testing.T) {
	t.Run("community registry returns as-is", func(t *testing.T) {
		ref, err := resolveCatalogRef("registry.modelcontextprotocol.io")
		require.NoError(t, err)
		assert.Equal(t, "registry.modelcontextprotocol.io", ref)
	})

	t.Run("community registry with tag returns as-is", func(t *testing.T) {
		ref, err := resolveCatalogRef("registry.modelcontextprotocol.io:latest")
		require.NoError(t, err)
		assert.Equal(t, "registry.modelcontextprotocol.io:latest", ref)
	})

	t.Run("OCI reference is normalized", func(t *testing.T) {
		ref, err := resolveCatalogRef("mcp/docker-mcp-catalog:latest")
		require.NoError(t, err)
		assert.Contains(t, ref, "mcp/docker-mcp-catalog:latest")
	})

	t.Run("invalid OCI reference returns error", func(t *testing.T) {
		_, err := resolveCatalogRef("INVALID:::::REF")
		require.Error(t, err)
	})
}

func TestInspectServer(t *testing.T) {
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
			err := InspectServer(ctx, dao, catalogObj.Ref, "my-server", workingset.OutputFormatJSON)
			require.NoError(t, err)
		})

		var server InspectResult
		err := json.Unmarshal([]byte(output), &server)
		require.NoError(t, err)
		assert.Equal(t, "my-server", server.Snapshot.Server.Name)
		assert.Equal(t, "docker/server1:v1", server.Image)
		assert.Empty(t, server.ReadmeContent)
	})

	t.Run("YAML format", func(t *testing.T) {
		output := captureStdout(t, func() {
			err := InspectServer(ctx, dao, catalogObj.Ref, "my-server", workingset.OutputFormatYAML)
			require.NoError(t, err)
		})

		var server InspectResult
		err := yaml.Unmarshal([]byte(output), &server)
		require.NoError(t, err)
		assert.Equal(t, "my-server", server.Snapshot.Server.Name)
		assert.Equal(t, "docker/server1:v1", server.Image)
		assert.Empty(t, server.ReadmeContent)
	})

	t.Run("HumanReadable format (uses YAML)", func(t *testing.T) {
		output := captureStdout(t, func() {
			err := InspectServer(ctx, dao, catalogObj.Ref, "my-server", workingset.OutputFormatHumanReadable)
			require.NoError(t, err)
		})

		var server InspectResult
		err := yaml.Unmarshal([]byte(output), &server)
		require.NoError(t, err)
		assert.Equal(t, "my-server", server.Snapshot.Server.Name)
		assert.Empty(t, server.ReadmeContent)
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
		err := InspectServer(ctx, dao, catalogObj.Ref, "my-server", workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var inspectResult InspectResult
	err = json.Unmarshal([]byte(output), &inspectResult)
	require.NoError(t, err)
	assert.Equal(t, "my-server", inspectResult.Snapshot.Server.Name)
	assert.Equal(t, "docker/server1:v1", inspectResult.Image)
	assert.Equal(t, readmeContent, inspectResult.ReadmeContent)
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

	err = InspectServer(ctx, dao, catalogObj.Ref, "nonexistent-server", workingset.OutputFormatJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server nonexistent-server not found in catalog test/catalog:latest")
}

func TestInspectServerCatalogNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := InspectServer(ctx, dao, "test/nonexistent:latest", "some-server", workingset.OutputFormatJSON)
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

	err = InspectServer(ctx, dao, catalogObj.Ref, "my-server", workingset.OutputFormat("unsupported"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format: unsupported")
}

func TestInspectServerInvalidCatalogRef(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := InspectServer(ctx, dao, ":::invalid-ref", "some-server", workingset.OutputFormatJSON)
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
			err := InspectServer(ctx, dao, catalogObj.Ref, "image-server", workingset.OutputFormatJSON)
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
			err := InspectServer(ctx, dao, catalogObj.Ref, "remote-server", workingset.OutputFormatJSON)
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
			err := InspectServer(ctx, dao, catalogObj.Ref, "registry-server", workingset.OutputFormatJSON)
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

func TestListServersCommunityRegistry(t *testing.T) {
	dao := setupTestDB(t)
	ctx := desktop.WithNoDockerDesktop(t.Context())

	// Seed a community registry catalog
	err := writeCommunityToDatabase(ctx, dao, "registry.modelcontextprotocol.io", map[string]catalog.Server{
		"community-server": {
			Name:  "community-server",
			Type:  "server",
			Image: "ghcr.io/example/community:latest",
		},
	})
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err := ListServers(ctx, dao, "registry.modelcontextprotocol.io", []string{}, workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "community-server")
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

func TestAddServersSuccess(t *testing.T) {
	t.Run("add docker image server", func(t *testing.T) {
		dao := setupTestDB(t)
		ctx := t.Context()

		// Create an initial catalog with one existing server
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

		ociService := mocks.NewMockOCIService(mocks.WithLocalImages([]mocks.MockImage{
			{
				Ref: "myimage:latest",
				Labels: map[string]string{
					"io.docker.server.metadata": "name: My Image\ntype: server",
				},
				DigestString: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			},
		}))

		output := captureStdout(t, func() {
			err := AddServers(ctx, dao, mocks.NewMockRegistryAPIClient(), ociService, catalogObj.Ref, []string{
				"docker://myimage:latest",
			})
			require.NoError(t, err)
		})

		assert.Contains(t, output, "Added 1 server(s)")

		// Verify server was added to the catalog
		dbCat2, err := dao.GetCatalog(ctx, catalogObj.Ref)
		require.NoError(t, err)
		cat := NewFromDb(dbCat2)
		assert.Len(t, cat.Servers, 2)
		assert.NotNil(t, cat.FindServer("existing-server"))
		assert.NotNil(t, cat.FindServer("My Image"))

		addedServer := cat.FindServer("My Image")
		assert.Equal(t, workingset.ServerTypeImage, addedServer.Type)
		assert.Equal(t, "myimage:latest", addedServer.Image)
	})

	t.Run("add multiple docker image servers", func(t *testing.T) {
		dao := setupTestDB(t)
		ctx := t.Context()

		catalogObj := Catalog{
			Ref: "test/catalog:latest",
			CatalogArtifact: CatalogArtifact{
				Title:   "Test Catalog",
				Servers: []Server{},
			},
		}

		dbCat, err := catalogObj.ToDb()
		require.NoError(t, err)
		err = dao.UpsertCatalog(ctx, dbCat)
		require.NoError(t, err)

		ociService := mocks.NewMockOCIService(mocks.WithLocalImages([]mocks.MockImage{
			{
				Ref: "server-a:latest",
				Labels: map[string]string{
					"io.docker.server.metadata": "name: Server A\ntype: server",
				},
				DigestString: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			{
				Ref: "server-b:latest",
				Labels: map[string]string{
					"io.docker.server.metadata": "name: Server B\ntype: server",
				},
				DigestString: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		}))

		output := captureStdout(t, func() {
			err := AddServers(ctx, dao, mocks.NewMockRegistryAPIClient(), ociService, catalogObj.Ref, []string{
				"docker://server-a:latest",
				"docker://server-b:latest",
			})
			require.NoError(t, err)
		})

		assert.Contains(t, output, "Added 2 server(s)")

		dbCat2, err := dao.GetCatalog(ctx, catalogObj.Ref)
		require.NoError(t, err)
		cat := NewFromDb(dbCat2)
		assert.Len(t, cat.Servers, 2)
		assert.NotNil(t, cat.FindServer("Server A"))
		assert.NotNil(t, cat.FindServer("Server B"))
	})

	t.Run("skip duplicate server", func(t *testing.T) {
		dao := setupTestDB(t)
		ctx := t.Context()

		catalogObj := Catalog{
			Ref: "test/catalog:latest",
			CatalogArtifact: CatalogArtifact{
				Title: "Test Catalog",
				Servers: []Server{
					{
						Type:  workingset.ServerTypeImage,
						Image: "docker/existing:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name: "My Image",
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

		ociService := mocks.NewMockOCIService(mocks.WithLocalImages([]mocks.MockImage{
			{
				Ref: "myimage:latest",
				Labels: map[string]string{
					"io.docker.server.metadata": "name: My Image\ntype: server",
				},
				DigestString: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			},
		}))

		output := captureStdout(t, func() {
			err := AddServers(ctx, dao, mocks.NewMockRegistryAPIClient(), ociService, catalogObj.Ref, []string{
				"docker://myimage:latest",
			})
			require.NoError(t, err)
		})

		assert.Contains(t, output, "No new servers added (all already exist)")

		// Verify catalog still has only one server
		dbCat2, err := dao.GetCatalog(ctx, catalogObj.Ref)
		require.NoError(t, err)
		cat := NewFromDb(dbCat2)
		assert.Len(t, cat.Servers, 1)
	})

	t.Run("add one new and skip one duplicate", func(t *testing.T) {
		dao := setupTestDB(t)
		ctx := t.Context()

		catalogObj := Catalog{
			Ref: "test/catalog:latest",
			CatalogArtifact: CatalogArtifact{
				Title: "Test Catalog",
				Servers: []Server{
					{
						Type:  workingset.ServerTypeImage,
						Image: "docker/existing:v1",
						Snapshot: &workingset.ServerSnapshot{
							Server: catalog.Server{
								Name: "Existing Server",
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

		ociService := mocks.NewMockOCIService(mocks.WithLocalImages([]mocks.MockImage{
			{
				Ref: "existing:v1",
				Labels: map[string]string{
					"io.docker.server.metadata": "name: Existing Server\ntype: server",
				},
				DigestString: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			{
				Ref: "new-server:latest",
				Labels: map[string]string{
					"io.docker.server.metadata": "name: New Server\ntype: server",
				},
				DigestString: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		}))

		output := captureStdout(t, func() {
			err := AddServers(ctx, dao, mocks.NewMockRegistryAPIClient(), ociService, catalogObj.Ref, []string{
				"docker://existing:v1",
				"docker://new-server:latest",
			})
			require.NoError(t, err)
		})

		assert.Contains(t, output, "already exists in catalog (skipping)")
		assert.Contains(t, output, "Added 1 server(s)")

		dbCat2, err := dao.GetCatalog(ctx, catalogObj.Ref)
		require.NoError(t, err)
		cat := NewFromDb(dbCat2)
		assert.Len(t, cat.Servers, 2)
		assert.NotNil(t, cat.FindServer("Existing Server"))
		assert.NotNil(t, cat.FindServer("New Server"))
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
