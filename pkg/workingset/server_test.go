package workingset

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
)

var oneServerError = "at least one server must be specified"

func TestAddOneServerToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	servers := []string{
		"docker://myimage:latest",
	}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "My Image", dbSet.Servers[0].Snapshot.Server.Name)
}

func TestAddMultipleServersToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	servers := []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "My Image", dbSet.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "Another Image", dbSet.Servers[1].Snapshot.Server.Name)
}

func TestAddNoServersToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	servers := []string{}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.Error(t, err)
	assert.Contains(t, err.Error(), oneServerError)
}

func TestRemoveOneServerFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	serverURI := "docker://myimage:latest"
	setID := "test-set"

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", "test-set", []string{
		serverURI,
	}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, setID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)

	err = RemoveServers(ctx, dao, setID, []string{
		"My Image",
	})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, setID)
	require.NoError(t, err)

	assert.Empty(t, dbSet.Servers)
}

func TestRemoveMultipleServersFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSetID := "test-set"

	servers := []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetID, "My Test Set", servers, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 2)

	err = RemoveServers(ctx, dao, workingSetID, []string{"My Image", "Another Image"})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers)
}

func TestRemoveOneOfManyServerFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSetID := "test-set"

	servers := []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetID, "My Test Set", servers, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 2)

	err = RemoveServers(ctx, dao, workingSetID, []string{"My Image"})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "Another Image", dbSet.Servers[0].Snapshot.Server.Name)
}

func TestRemoveNoServersFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSetID := "test-set"

	servers := []string{
		"docker://myimage:latest",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetID, "My Test Set", servers, []string{})
	require.NoError(t, err)

	err = RemoveServers(ctx, dao, workingSetID, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), oneServerError)
}

func TestRemoveServersCombinedNames(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSetID := "test-set"

	// Create a profile with two servers
	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetID, "My Test Set", []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 2)

	// Simulate what the cobra handler produces when mixing positional args and --name flags:
	// e.g. `docker mcp profile server remove test-set "My Image" --name "Another Image"`
	// would yield allNames = ["My Image", "Another Image"]
	allNames := make([]string, 0, 2)
	allNames = append(allNames, "My Image")      // from positional arg
	allNames = append(allNames, "Another Image") // from --name flag

	err = RemoveServers(ctx, dao, workingSetID, allNames)
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers)
}

func TestAddServersFromCatalog(t *testing.T) {
	tests := []struct {
		name           string
		catalogServers []testCatalogServer
		serverNames    []string
		validateServer func(t *testing.T, servers []db.Server)
	}{
		{
			name: "image servers",
			catalogServers: []testCatalogServer{
				{
					name:       "catalog-server-1",
					serverType: "image",
					image:      "catalog-image-1:latest",
					tools:      []string{"tool1", "tool2"},
				},
				{
					name:       "catalog-server-2",
					serverType: "image",
					image:      "catalog-image-2:latest",
					tools:      []string{"tool1"},
				},
			},
			serverNames: []string{"catalog-server-1", "catalog-server-2"},
			validateServer: func(t *testing.T, servers []db.Server) {
				t.Helper()
				assert.Equal(t, "image", servers[0].Type)
				assert.Equal(t, "catalog-image-1:latest", servers[0].Image)
				assert.Equal(t, "catalog-server-1", servers[0].Snapshot.Server.Name)
				assert.Equal(t, []string{"tool1", "tool2"}, servers[0].Tools)
				assert.Equal(t, "default", servers[0].Secrets)
				assert.Equal(t, "image", servers[1].Type)
				assert.Equal(t, "catalog-image-2:latest", servers[1].Image)
				assert.Equal(t, "catalog-server-2", servers[1].Snapshot.Server.Name)
				assert.Equal(t, []string{"tool1"}, servers[1].Tools)
				assert.Equal(t, "default", servers[1].Secrets)
			},
		},
		{
			name: "basic remotes with SSE transport",
			catalogServers: []testCatalogServer{
				{
					name:       "ais-fleet",
					serverType: "remote",
					endpoint:   "https://mcp.aisfleet.com/sse",
					tools:      []string{"tool1"},
				},
				{
					name:       "ais-fleet-2",
					serverType: "remote",
					endpoint:   "https://mcp.aisfleet-2.com/sse",
					tools:      []string{"tool2"},
				},
			},
			serverNames: []string{"ais-fleet", "ais-fleet-2"},
			validateServer: func(t *testing.T, servers []db.Server) {
				t.Helper()
				assert.Equal(t, "remote", servers[0].Type)
				assert.Equal(t, "https://mcp.aisfleet.com/sse", servers[0].Endpoint)
				assert.Equal(t, "ais-fleet", servers[0].Snapshot.Server.Name)
				assert.Len(t, servers[0].Tools, 1)
				assert.Equal(t, "default", servers[0].Secrets)

				assert.Equal(t, "remote", servers[1].Type)
				assert.Equal(t, "https://mcp.aisfleet-2.com/sse", servers[1].Endpoint)
				assert.Equal(t, "ais-fleet-2", servers[1].Snapshot.Server.Name)
				assert.Len(t, servers[1].Tools, 1)
				assert.Equal(t, "default", servers[1].Secrets)
			},
		},
		{
			name: "remote with streamable-http and authorization header",
			catalogServers: []testCatalogServer{
				{
					name:       "apify-remote",
					serverType: "remote",
					endpoint:   "https://mcp.apify.com",
					tools:      []string{"apify-tool1", "apify-tool2"},
				},
			},
			serverNames: []string{"apify-remote"},
			validateServer: func(t *testing.T, servers []db.Server) {
				t.Helper()
				assert.Equal(t, "remote", servers[0].Type)
				assert.Equal(t, "https://mcp.apify.com", servers[0].Endpoint)
				assert.Equal(t, "apify-remote", servers[0].Snapshot.Server.Name)
				assert.Len(t, servers[0].Tools, 2)
				assert.Equal(t, "default", servers[0].Secrets)
			},
		},
		{
			name: "remote with OAuth",
			catalogServers: []testCatalogServer{
				{
					name:       "asana",
					serverType: "remote",
					endpoint:   "https://asana.com/api/mcp/v1/sse",
					tools:      []string{"asana-task-create", "asana-task-update"},
				},
			},
			serverNames: []string{"asana"},
			validateServer: func(t *testing.T, servers []db.Server) {
				t.Helper()
				assert.Equal(t, "remote", servers[0].Type)
				assert.Equal(t, "https://asana.com/api/mcp/v1/sse", servers[0].Endpoint)
				assert.Equal(t, "asana", servers[0].Snapshot.Server.Name)
				assert.Len(t, servers[0].Tools, 2)
				assert.Equal(t, "default", servers[0].Secrets)
			},
		},
		{
			name: "remote with dynamic tools",
			catalogServers: []testCatalogServer{
				{
					name:       "cloudflare-audit-logs",
					serverType: "remote",
					endpoint:   "https://auditlogs.mcp.cloudflare.com/sse",
					tools:      []string{},
				},
			},
			serverNames: []string{"cloudflare-audit-logs"},
			validateServer: func(t *testing.T, servers []db.Server) {
				t.Helper()
				assert.Equal(t, "remote", servers[0].Type)
				assert.Equal(t, "https://auditlogs.mcp.cloudflare.com/sse", servers[0].Endpoint)
				assert.Equal(t, "cloudflare-audit-logs", servers[0].Snapshot.Server.Name)
				assert.Empty(t, servers[0].Tools)
				assert.Equal(t, "default", servers[0].Secrets)
			},
		},
		{
			name: "remote with static tools list",
			catalogServers: []testCatalogServer{
				{
					name:       "gitmcp",
					serverType: "remote",
					endpoint:   "https://gitmcp.io/docs",
					tools:      []string{"match_common_libs_owner_repo_mapping", "fetch_generic_documentation"},
				},
			},
			serverNames: []string{"gitmcp"},
			validateServer: func(t *testing.T, servers []db.Server) {
				t.Helper()
				assert.Equal(t, "remote", servers[0].Type)
				assert.Equal(t, "https://gitmcp.io/docs", servers[0].Endpoint)
				assert.Equal(t, "gitmcp", servers[0].Snapshot.Server.Name)
				assert.Len(t, servers[0].Tools, 2)
				assert.Equal(t, "default", servers[0].Secrets)
			},
		},
		{
			name: "remote with SSE, headers, and secrets",
			catalogServers: []testCatalogServer{
				{
					name:       "dodo-payments",
					serverType: "remote",
					endpoint:   "https://mcp.dodopayments.com/sse",
					tools:      []string{"payment-create", "payment-refund"},
				},
			},
			serverNames: []string{"dodo-payments"},
			validateServer: func(t *testing.T, servers []db.Server) {
				t.Helper()
				assert.Equal(t, "remote", servers[0].Type)
				assert.Equal(t, "https://mcp.dodopayments.com/sse", servers[0].Endpoint)
				assert.Equal(t, "dodo-payments", servers[0].Snapshot.Server.Name)
				assert.Len(t, servers[0].Tools, 2)
				assert.Equal(t, "default", servers[0].Secrets)
			},
		},
		{
			name: "remote documentation server (no auth)",
			catalogServers: []testCatalogServer{
				{
					name:       "cloudflare-docs",
					serverType: "remote",
					endpoint:   "https://docs.mcp.cloudflare.com/sse",
					tools:      []string{"search_cloudflare_documentation", "migrate_pages_to_workers_guide"},
				},
			},
			serverNames: []string{"cloudflare-docs"},
			validateServer: func(t *testing.T, servers []db.Server) {
				t.Helper()
				assert.Equal(t, "remote", servers[0].Type)
				assert.Equal(t, "https://docs.mcp.cloudflare.com/sse", servers[0].Endpoint)
				assert.Equal(t, "cloudflare-docs", servers[0].Snapshot.Server.Name)
				assert.Len(t, servers[0].Tools, 2)
				assert.Equal(t, "default", servers[0].Secrets)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dao := setupTestDB(t)
			ctx := t.Context()

			// Create a catalog with test servers
			catalog := createTestCatalog(t, dao, tt.catalogServers)

			// Create a working set
			err := dao.CreateWorkingSet(ctx, db.WorkingSet{
				ID:      "test-set",
				Name:    "Test Working Set",
				Servers: db.ServerList{},
				Secrets: db.SecretMap{
					"default": {Provider: "docker-desktop-store"},
				},
			})
			require.NoError(t, err)

			// Build the catalog URL with server names
			serverNamesJoined := ""
			for i, name := range tt.serverNames {
				if i > 0 {
					serverNamesJoined += "+"
				}
				serverNamesJoined += name
			}

			// Add servers from catalog
			err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"catalog://" + catalog.Ref + "/" + serverNamesJoined})
			require.NoError(t, err)

			// Verify servers were added
			dbSet, err := dao.GetWorkingSet(ctx, "test-set")
			require.NoError(t, err)
			require.NotNil(t, dbSet)
			assert.Len(t, dbSet.Servers, len(tt.serverNames))
			tt.validateServer(t, dbSet.Servers)
		})
	}
}

func TestAddServersMixedDirectAndCatalog(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "catalog-server-1",
			serverType: "image",
			image:      "catalog-image-1:latest",
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	// Add both direct servers and catalog servers
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"docker://myimage:latest", "catalog://" + catalog.Ref + "/catalog-server-1"})
	require.NoError(t, err)

	// Verify both types of servers were added
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Len(t, dbSet.Servers, 2)

	// First should be the direct server
	assert.Equal(t, "My Image", dbSet.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "myimage:latest", dbSet.Servers[0].Image)

	// Second should be from catalog
	assert.Equal(t, "catalog-server-1", dbSet.Servers[1].Snapshot.Server.Name)
	assert.Equal(t, "catalog-image-1:latest", dbSet.Servers[1].Image)
}

func TestAddServersFromCatalogMissingServer(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "catalog-server-1",
			serverType: "image",
			image:      "catalog-image-1:latest",
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to add a server that doesn't exist in the catalog
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"catalog://" + catalog.Ref + "/catalog-server-1+nonexistent-server"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "servers matching the following patterns were not found in catalog: [nonexistent-server]")
	assert.Contains(t, err.Error(), "nonexistent-server")
}

func TestAddServersFromCatalogInvalidDigest(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to add servers from a non-existent catalog
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"catalog://invalid-name/some-server"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog invalid-name:latest not found")
}

func TestAddServersFromCatalogServersWithoutCatalog(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to add servers from a non-existent catalog
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"catalog://some-server"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid server value: invalid catalog URL: catalog://some-server")
}

func TestAddServersFromCatalogWithoutDefaultSecret(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "catalog-server-1",
			serverType: "image",
			image:      "catalog-image-1:latest",
		},
	})

	// Create a working set without default secret
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{
			"custom": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	// Add server from catalog
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"catalog://" + catalog.Ref + "/catalog-server-1"})
	require.NoError(t, err)

	// Verify server was added without default secret
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Len(t, dbSet.Servers, 1)
	assert.Empty(t, dbSet.Servers[0].Secrets) // Should be empty string when no default
}

func TestAddServersFromCatalogEmptyCatalogServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog
	createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "catalog-server-1",
			serverType: "image",
			image:      "catalog-image-1:latest",
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to add with catalog ref but empty server list
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"catalog://test/catalog:latest"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid server value: catalog test:latest not found")
}

// Helper types and functions for catalog tests
type testCatalogServer struct {
	name       string
	serverType string
	image      string
	source     string
	endpoint   string
	tools      []string
	snapshot   *db.ServerSnapshot
}

func createTestCatalog(t *testing.T, dao db.DAO, servers []testCatalogServer) db.Catalog {
	t.Helper()

	catalogServers := make([]db.CatalogServer, len(servers))
	for i, server := range servers {
		catalogServers[i] = db.CatalogServer{
			ServerType: server.serverType,
			Tools:      server.tools,
			Source:     server.source,
			Image:      server.image,
			Endpoint:   server.endpoint,
		}

		// Use provided snapshot if available, otherwise create default
		if server.snapshot != nil {
			catalogServers[i].Snapshot = server.snapshot
		} else {
			catalogServers[i].Snapshot = &db.ServerSnapshot{
				Server: catalog.Server{
					Name:  server.name,
					Type:  "server",
					Image: server.image,
				},
			}
		}
	}

	catalog := db.Catalog{
		Ref:     "test/catalog:latest",
		Digest:  "test-digest",
		Title:   "Test Catalog",
		Source:  "https://example.com/catalog",
		Servers: catalogServers,
	}

	err := dao.UpsertCatalog(t.Context(), catalog)
	require.NoError(t, err)

	return catalog
}

func TestAddServersFromCatalogUsingGlobPattern(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with multiple servers that share a naming pattern
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "cloudflare-docs",
			serverType: "remote",
			endpoint:   "https://docs.mcp.cloudflare.com/sse",
		},
		{
			name:       "cloudflare-audit-logs",
			serverType: "remote",
			endpoint:   "https://auditlogs.mcp.cloudflare.com/sse",
		},
		{
			name:       "cloudflare-workers",
			serverType: "remote",
			endpoint:   "https://workers.mcp.cloudflare.com/sse",
		},
		{
			name:       "other-server",
			serverType: "image",
			image:      "other-image:latest",
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	// Add servers using a glob pattern that matches all cloudflare-* servers
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"catalog://" + catalog.Ref + "/cloudflare-*"})
	require.NoError(t, err)

	// Verify only the cloudflare servers were added (3 of them)
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Len(t, dbSet.Servers, 3)

	// Verify the correct servers were added
	serverNames := make([]string, len(dbSet.Servers))
	for i, server := range dbSet.Servers {
		serverNames[i] = server.Snapshot.Server.Name
	}
	assert.Contains(t, serverNames, "cloudflare-docs")
	assert.Contains(t, serverNames, "cloudflare-audit-logs")
	assert.Contains(t, serverNames, "cloudflare-workers")
	assert.NotContains(t, serverNames, "other-server")
}

func TestAddServersFromCatalogUsingMultipleGlobPatterns(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with servers that match different patterns
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "cloudflare-docs",
			serverType: "remote",
			endpoint:   "https://docs.mcp.cloudflare.com/sse",
		},
		{
			name:       "cloudflare-audit-logs",
			serverType: "remote",
			endpoint:   "https://auditlogs.mcp.cloudflare.com/sse",
		},
		{
			name:       "github-repos",
			serverType: "remote",
			endpoint:   "https://github.com/mcp/repos",
		},
		{
			name:       "github-issues",
			serverType: "remote",
			endpoint:   "https://github.com/mcp/issues",
		},
		{
			name:       "other-server",
			serverType: "image",
			image:      "other-image:latest",
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	// Add servers using multiple glob patterns joined with +
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"catalog://" + catalog.Ref + "/cloudflare-*+github-*"})
	require.NoError(t, err)

	// Verify 4 servers were added (2 cloudflare + 2 github)
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Len(t, dbSet.Servers, 4)

	// Verify the correct servers were added
	serverNames := make([]string, len(dbSet.Servers))
	for i, server := range dbSet.Servers {
		serverNames[i] = server.Snapshot.Server.Name
	}
	assert.Contains(t, serverNames, "cloudflare-docs")
	assert.Contains(t, serverNames, "cloudflare-audit-logs")
	assert.Contains(t, serverNames, "github-repos")
	assert.Contains(t, serverNames, "github-issues")
	assert.NotContains(t, serverNames, "other-server")
}

func TestAddServersFromCatalogUsingMultipleGlobPatternsWithDuplicates(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with servers that match different patterns
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "cloudflare-docs",
			serverType: "remote",
			endpoint:   "https://docs.mcp.cloudflare.com/sse",
		},
		{
			name:       "cloudflare-audit-logs",
			serverType: "remote",
			endpoint:   "https://auditlogs.mcp.cloudflare.com/sse",
		},
		{
			name:       "other-server",
			serverType: "image",
			image:      "other-image:latest",
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	// Add servers using multiple glob patterns joined with +
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"catalog://" + catalog.Ref + "/cloudflare-*+cloudflare-*"})
	require.NoError(t, err)

	// Verify 2 servers were added
	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Len(t, dbSet.Servers, 2)

	// Verify the correct servers were added
	serverNames := make([]string, len(dbSet.Servers))
	for i, server := range dbSet.Servers {
		serverNames[i] = server.Snapshot.Server.Name
	}
	assert.Contains(t, serverNames, "cloudflare-docs")
	assert.Contains(t, serverNames, "cloudflare-audit-logs")
	assert.NotContains(t, serverNames, "other-server")
}

func TestAddServersFromCatalogGlobPatternNoMatch(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with servers
	catalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "cloudflare-docs",
			serverType: "remote",
			endpoint:   "https://docs.mcp.cloudflare.com/sse",
		},
	})

	// Create a working set
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Try to add servers using a glob pattern that doesn't match anything
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{"catalog://" + catalog.Ref + "/nonexistent-*"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "servers matching the following patterns were not found in catalog")
	assert.Contains(t, err.Error(), "nonexistent-*")
}

func TestListServersNoFilters(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "set-1", "Set 1", []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}, []string{})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := ListServers(ctx, dao, []string{}, OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
}

func TestListServersFilterByName(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "set-1", "Set 1", []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}, []string{})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := ListServers(ctx, dao, []string{"name=My"}, OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, "My Image", results[0].Servers[0].Snapshot.Server.Name)
}

func TestListServersFilterByNameCaseInsensitive(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "set-1", "Set 1", []string{
		"docker://myimage:latest",
	}, []string{})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := ListServers(ctx, dao, []string{"name=my image"}, OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, "My Image", results[0].Servers[0].Snapshot.Server.Name)
}

func TestListServersFilterByWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "set-1", "Set 1", []string{
		"docker://myimage:latest",
	}, []string{})
	require.NoError(t, err)

	err = Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "set-2", "Set 2", []string{
		"docker://anotherimage:v1.0",
	}, []string{})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := ListServers(ctx, dao, []string{"profile=set-2"}, OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "set-2", results[0].ID)
	assert.Equal(t, "Another Image", results[0].Servers[0].Snapshot.Server.Name)
}

func TestListServersFilterByBothNameAndWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "set-1", "Set 1", []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}, []string{})
	require.NoError(t, err)

	err = Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "set-2", "Set 2", []string{
		"docker://myimage:latest",
	}, []string{})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := ListServers(ctx, dao, []string{"profile=set-1", "name=Another"}, OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, "Another Image", results[0].Servers[0].Snapshot.Server.Name)
}

func TestListServersFilterNoMatches(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "set-1", "Set 1", []string{
		"docker://myimage:latest",
	}, []string{})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := ListServers(ctx, dao, []string{"name=nonexistent"}, OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestListServersInvalidFilter(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := ListServers(ctx, dao, []string{"invalid"}, OutputFormatJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filter format")
}

func TestListServersUnsupportedFilterKey(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := ListServers(ctx, dao, []string{"unsupported=value"}, OutputFormatJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported filter key")
}

func TestAddServerUpsertReplacesExisting(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a profile with one server
	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", "Test", []string{
		"docker://myimage:latest",
	}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "My Image", dbSet.Servers[0].Snapshot.Server.Name)

	// Add the same server again -- should upsert (replace), not error
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{
		"docker://myimage:latest",
	})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "My Image", dbSet.Servers[0].Snapshot.Server.Name)
}

func TestAddServerUpsertPreservesOtherServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a profile with two servers
	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", "Test", []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 2)

	// Upsert only the first server
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{
		"docker://myimage:latest",
	})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 2)

	// "Another Image" should still be present (and first, since it was not replaced)
	assert.Equal(t, "Another Image", dbSet.Servers[0].Snapshot.Server.Name)
	// "My Image" should be at the end (re-added after filtering)
	assert.Equal(t, "My Image", dbSet.Servers[1].Snapshot.Server.Name)
}

func TestAddServerUpsertFromCatalog(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a catalog with a server
	testCatalog := createTestCatalog(t, dao, []testCatalogServer{
		{
			name:       "catalog-server-1",
			serverType: "image",
			image:      "catalog-image-1:latest",
		},
	})

	// Create a profile with the catalog server
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{
			"default": {Provider: "docker-desktop-store"},
		},
	})
	require.NoError(t, err)

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{
		"catalog://" + testCatalog.Ref + "/catalog-server-1",
	})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "catalog-server-1", dbSet.Servers[0].Snapshot.Server.Name)

	// Now create a second catalog version with an updated server of the same name
	updatedCatalog := db.Catalog{
		Ref:    "test/catalog-v2:latest",
		Digest: "test-digest-v2",
		Title:  "Test Catalog V2",
		Source: "https://example.com/catalog-v2",
		Servers: []db.CatalogServer{
			{
				ServerType: "image",
				Image:      "catalog-image-1:v2",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name:  "catalog-server-1",
						Type:  "server",
						Image: "catalog-image-1:v2",
					},
				},
			},
		},
	}
	err = dao.UpsertCatalog(ctx, updatedCatalog)
	require.NoError(t, err)

	// Upsert the server from the new catalog version
	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", []string{
		"catalog://" + updatedCatalog.Ref + "/catalog-server-1",
	})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "catalog-server-1", dbSet.Servers[0].Snapshot.Server.Name)
	// Verify it's the updated version
	assert.Equal(t, "catalog-image-1:v2", dbSet.Servers[0].Image)
}
