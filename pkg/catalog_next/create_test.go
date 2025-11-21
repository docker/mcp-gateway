package catalognext

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func TestCreateFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set first
	ws := db.WorkingSet{
		ID:   "test-ws",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  string(workingset.ServerTypeImage),
				Image: "docker/test:latest",
				Tools: []string{"tool1", "tool2"},
			},
			{
				Type:   string(workingset.ServerTypeRegistry),
				Source: "https://example.com/server",
				Tools:  []string{"tool3"},
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Capture stdout to verify the output message
	output := captureStdout(t, func() {
		err := Create(ctx, dao, "test/catalog:latest", "test-ws", "", "My Catalog")
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Catalog test/catalog:latest created")

	// Verify the catalog was created
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	catalog := NewFromDb(&catalogs[0])
	assert.Equal(t, "My Catalog", catalog.Title)
	assert.Equal(t, "profile:test-ws", catalog.Source)
	assert.Len(t, catalog.Servers, 2)

	// Verify servers were copied correctly
	assert.Equal(t, workingset.ServerTypeImage, catalog.Servers[0].Type)
	assert.Equal(t, "docker/test:latest", catalog.Servers[0].Image)
	assert.Equal(t, []string{"tool1", "tool2"}, catalog.Servers[0].Tools)

	assert.Equal(t, workingset.ServerTypeRegistry, catalog.Servers[1].Type)
	assert.Equal(t, "https://example.com/server", catalog.Servers[1].Source)
	assert.Equal(t, []string{"tool3"}, catalog.Servers[1].Tools)
}

func TestCreateFromWorkingSetNormalizedRef(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set first
	ws := db.WorkingSet{
		ID:      "test-ws",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Capture stdout to verify the output message
	output := captureStdout(t, func() {
		err := Create(ctx, dao, "docker.io/test/catalog:latest", "test-ws", "", "My Catalog")
		require.NoError(t, err)
	})

	// Verify output message - docker.io prefix is normalized away
	assert.Contains(t, output, "Catalog test/catalog:latest created")

	// Verify the catalog was created
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	catalog := NewFromDb(&catalogs[0])
	assert.Equal(t, "test/catalog:latest", catalog.Ref)
}

func TestCreateFromWorkingSetRejectsDigestReference(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	ws := db.WorkingSet{
		ID:      "test-ws",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	digestRef := "test/catalog@sha256:0000000000000000000000000000000000000000000000000000000000000000"
	err = Create(ctx, dao, digestRef, "test-ws", "", "My Catalog")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reference must be a valid OCI reference without a digest")
}

func TestCreateFromWorkingSetWithEmptyName(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	ws := db.WorkingSet{
		ID:   "test-ws",
		Name: "Original Working Set Name",
		Servers: db.ServerList{
			{
				Type:  string(workingset.ServerTypeImage),
				Image: "docker/test:latest",
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create catalog without providing a title (should use working set name)
	captureStdout(t, func() {
		err := Create(ctx, dao, "test/catalog2:latest", "test-ws", "", "")
		require.NoError(t, err)
	})

	// Verify the catalog was created with working set name as title
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	catalog := NewFromDb(&catalogs[0])
	assert.Equal(t, "Original Working Set Name", catalog.Title)
}

func TestCreateFromWorkingSetNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := Create(ctx, dao, "test/catalog3:latest", "nonexistent-ws", "", "Test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "profile nonexistent-ws not found")
}

func TestCreateFromWorkingSetDuplicate(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set
	ws := db.WorkingSet{
		ID:   "test-ws",
		Name: "Test",
		Servers: db.ServerList{
			{
				Type:  string(workingset.ServerTypeImage),
				Image: "docker/test:latest",
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create catalog from working set
	captureStdout(t, func() {
		err := Create(ctx, dao, "test/catalog4:latest", "test-ws", "", "Test")
		require.NoError(t, err)
	})

	// Create with same ref again - should succeed and replace (upsert behavior)
	err = Create(ctx, dao, "test/catalog4:latest", "test-ws", "", "Test Updated")
	require.NoError(t, err)

	// Verify it was updated
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)
	catalog := NewFromDb(&catalogs[0])
	assert.Equal(t, "Test Updated", catalog.Title)
}

func TestCreateFromWorkingSetWithSnapshot(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with snapshot
	snapshot := &db.ServerSnapshot{
		Server: catalog.Server{
			Name:        "test-server",
			Description: "Test server",
		},
	}

	ws := db.WorkingSet{
		ID:   "test-ws",
		Name: "Test",
		Servers: db.ServerList{
			{
				Type:     string(workingset.ServerTypeImage),
				Image:    "docker/test:latest",
				Snapshot: snapshot,
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create catalog from working set
	captureStdout(t, func() {
		err := Create(ctx, dao, "test/catalog5:latest", "test-ws", "", "Test")
		require.NoError(t, err)
	})

	// Verify snapshot was preserved
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	catalog := NewFromDb(&catalogs[0])
	require.NotNil(t, catalog.Servers[0].Snapshot)
	assert.Equal(t, "test-server", catalog.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "Test server", catalog.Servers[0].Snapshot.Server.Description)
}

func TestCreateFromWorkingSetEmptyServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with no servers
	ws := db.WorkingSet{
		ID:      "empty-ws",
		Name:    "Empty",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create catalog from empty working set
	captureStdout(t, func() {
		err := Create(ctx, dao, "test/catalog7:latest", "empty-ws", "", "Empty Catalog")
		require.NoError(t, err)
	})

	retrieved, err := dao.GetCatalog(ctx, "test/catalog7:latest")
	require.NoError(t, err)

	catalog := NewFromDb(retrieved)
	assert.Equal(t, "Empty Catalog", catalog.Title)
	assert.Empty(t, catalog.Servers)
}

func TestCreateFromWorkingSetPreservesAllServerFields(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a working set with all possible server fields
	ws := db.WorkingSet{
		ID:   "detailed-ws",
		Name: "Detailed",
		Servers: db.ServerList{
			{
				Type:   string(workingset.ServerTypeRegistry),
				Source: "https://example.com/api",
				Tools:  []string{"read", "write", "delete"},
				Config: map[string]any{
					"timeout": 30,
					"retries": 3,
				},
				Secrets: "api-secrets",
			},
			{
				Type:  string(workingset.ServerTypeImage),
				Image: "mycompany/myserver:v1.2.3",
				Tools: []string{"deploy"},
			},
		},
		Secrets: db.SecretMap{},
	}

	err := dao.CreateWorkingSet(ctx, ws)
	require.NoError(t, err)

	// Create catalog
	captureStdout(t, func() {
		err := Create(ctx, dao, "test/catalog8:latest", "detailed-ws", "", "Detailed Catalog")
		require.NoError(t, err)
	})

	retrieved, err := dao.GetCatalog(ctx, "test/catalog8:latest")
	require.NoError(t, err)
	catalog := NewFromDb(retrieved)

	assert.Equal(t, "Detailed Catalog", catalog.Title)
	assert.Equal(t, "profile:detailed-ws", catalog.Source)
	assert.Len(t, catalog.Servers, 2)

	// Check registry server
	assert.Equal(t, workingset.ServerTypeRegistry, catalog.Servers[0].Type)
	assert.Equal(t, "https://example.com/api", catalog.Servers[0].Source)
	assert.Equal(t, []string{"read", "write", "delete"}, catalog.Servers[0].Tools)

	// Check image server
	assert.Equal(t, workingset.ServerTypeImage, catalog.Servers[1].Type)
	assert.Equal(t, "mycompany/myserver:v1.2.3", catalog.Servers[1].Image)
	assert.Equal(t, []string{"deploy"}, catalog.Servers[1].Tools)
}

func TestCreateFromLegacyCatalog(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a temporary legacy catalog file
	tempDir := t.TempDir()
	catalogFile := filepath.Join(tempDir, "test-catalog.yaml")

	legacyCatalogYAML := `name: test-catalog
registry:
  server1:
    title: "Test Server 1"
    type: "server"
    image: "docker/test-server:latest"
    description: "A test server"
  server2:
    title: "Test Server 2"
    type: "server"
    image: "mycompany/another-server:v1.0"
    description: "Another test server"
`

	err := os.WriteFile(catalogFile, []byte(legacyCatalogYAML), 0o644)
	require.NoError(t, err)

	// Create catalog from legacy catalog
	output := captureStdout(t, func() {
		err := Create(ctx, dao, "test/imported:latest", "", catalogFile, "Imported Catalog")
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Catalog test/imported:latest created")

	// Verify the catalog was created
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	catalog := NewFromDb(&catalogs[0])
	assert.Equal(t, "Imported Catalog", catalog.Title)
	assert.Equal(t, "legacy-catalog:test-catalog", catalog.Source)
	assert.Len(t, catalog.Servers, 2)

	// Verify servers are sorted by name (name is the map key, not the name field)
	assert.Equal(t, "server1", catalog.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "Test Server 1", catalog.Servers[0].Snapshot.Server.Title)
	assert.Equal(t, workingset.ServerTypeImage, catalog.Servers[0].Type)
	assert.Equal(t, "docker/test-server:latest", catalog.Servers[0].Image)
	assert.Equal(t, "A test server", catalog.Servers[0].Snapshot.Server.Description)

	assert.Equal(t, "server2", catalog.Servers[1].Snapshot.Server.Name)
	assert.Equal(t, "Test Server 2", catalog.Servers[1].Snapshot.Server.Title)
	assert.Equal(t, workingset.ServerTypeImage, catalog.Servers[1].Type)
	assert.Equal(t, "mycompany/another-server:v1.0", catalog.Servers[1].Image)
	assert.Equal(t, "Another test server", catalog.Servers[1].Snapshot.Server.Description)
}

func TestCreateFromLegacyCatalogWithRemoveExistingWithSameContent(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a temporary legacy catalog file
	tempDir := t.TempDir()
	catalogFile := filepath.Join(tempDir, "test-catalog.yaml")

	legacyCatalogYAML := `name: test-catalog
registry:
  server1:
    name: "Test Server 1"
    type: "server"
    image: "docker/test-server:latest"
`

	err := os.WriteFile(catalogFile, []byte(legacyCatalogYAML), 0o644)
	require.NoError(t, err)

	// Create catalog from legacy catalog (first time)
	output1 := captureStdout(t, func() {
		err := Create(ctx, dao, "test/legacy3:latest", "", catalogFile, "Test Catalog")
		require.NoError(t, err)
	})
	assert.Contains(t, output1, "test/legacy3:latest created")

	// Get the first catalog's digest
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	require.Len(t, catalogs, 1)
	firstDigest := catalogs[0].Digest

	// Create with same ref again (upsert) - should replace
	output2 := captureStdout(t, func() {
		err := Create(ctx, dao, "test/legacy3:latest", "", catalogFile, "Test Catalog")
		require.NoError(t, err)
	})
	assert.Contains(t, output2, "test/legacy3:latest created")

	// Verify there's still only one catalog
	catalogs, err = dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	// Verify it has the same digest (same content)
	catalog := NewFromDb(&catalogs[0])
	assert.Equal(t, firstDigest, catalog.Digest)
	assert.Equal(t, "Test Catalog", catalog.Title)
	assert.Equal(t, "legacy-catalog:test-catalog", catalog.Source)
}

func TestCreateFromLegacyCatalogWithRemoveExistingWithDifferentContent(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a temporary legacy catalog file
	tempDir := t.TempDir()
	catalogFile := filepath.Join(tempDir, "test-catalog.yaml")

	legacyCatalogYAML := `name: test-catalog
registry:
  server1:
    title: "Test Server 1"
    type: "server"
    image: "docker/test-server:v1"
`

	err := os.WriteFile(catalogFile, []byte(legacyCatalogYAML), 0o644)
	require.NoError(t, err)

	// Create catalog from legacy catalog (first time)
	output1 := captureStdout(t, func() {
		err := Create(ctx, dao, "test/legacy4:latest", "", catalogFile, "Test Catalog")
		require.NoError(t, err)
	})
	assert.Contains(t, output1, "test/legacy4:latest created")

	// Get the first catalog's digest
	catalogs, err := dao.ListCatalogs(ctx)
	require.NoError(t, err)
	require.Len(t, catalogs, 1)
	firstDigest := catalogs[0].Digest

	legacyCatalogYAML = `name: test-catalog
registry:
  server1:
    title: "Test Server 1"
    type: "server"
    image: "docker/test-server:v2"
`

	err = os.WriteFile(catalogFile, []byte(legacyCatalogYAML), 0o644)
	require.NoError(t, err)

	// Create with same ref again (upsert) - should replace with new content
	output2 := captureStdout(t, func() {
		err := Create(ctx, dao, "test/legacy4:latest", "", catalogFile, "Test Catalog")
		require.NoError(t, err)
	})
	assert.Contains(t, output2, "test/legacy4:latest created")

	// Verify there's still only one catalog
	catalogs, err = dao.ListCatalogs(ctx)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)

	// Verify it has a different digest (different content)
	catalog := NewFromDb(&catalogs[0])
	assert.NotEqual(t, firstDigest, catalog.Digest)
	assert.Equal(t, "Test Catalog", catalog.Title)
	assert.Equal(t, "legacy-catalog:test-catalog", catalog.Source)
}

func TestCreateFromLegacyCatalogWithRemotes(t *testing.T) {
	tests := []struct {
		name           string
		serverYAML     string
		expectedType   workingset.ServerType
		validateServer func(t *testing.T, server *catalog.Server)
	}{
		{
			name: "basic remote with SSE transport",
			serverYAML: `    title: "AIS Fleet"
    type: remote
    remote:
      transport_type: sse
      url: https://mcp.aisfleet.com/sse`,
			expectedType: workingset.ServerTypeRemote,
			validateServer: func(t *testing.T, server *catalog.Server) {
				t.Helper()
				assert.Equal(t, "sse", server.Remote.Transport)
				assert.Equal(t, "https://mcp.aisfleet.com/sse", server.Remote.URL)
				assert.Empty(t, server.Remote.Headers)
				assert.Empty(t, server.Secrets)
				assert.Nil(t, server.OAuth)
			},
		},
		{
			name: "remote with streamable-http and authorization header",
			serverYAML: `    title: "Apify Remote"
    type: remote
    remote:
      transport_type: streamable-http
      url: https://mcp.apify.com
      headers:
        Authorization: "Bearer ${APIFY_API_KEY}"
    secrets:
      - name: apify.api_key
        env: APIFY_API_KEY
        example: <YOUR_API_KEY>`,
			expectedType: workingset.ServerTypeRemote,
			validateServer: func(t *testing.T, server *catalog.Server) {
				t.Helper()
				assert.Equal(t, "streamable-http", server.Remote.Transport)
				assert.Equal(t, "https://mcp.apify.com", server.Remote.URL)
				assert.Equal(t, "Bearer ${APIFY_API_KEY}", server.Remote.Headers["Authorization"])
				assert.Len(t, server.Secrets, 1)
				assert.Equal(t, "apify.api_key", server.Secrets[0].Name)
				assert.Equal(t, "APIFY_API_KEY", server.Secrets[0].Env)
			},
		},
		{
			name: "remote with OAuth",
			serverYAML: `    title: "Asana"
    type: remote
    remote:
      transport_type: streamable-http
      url: https://asana.com/api/mcp/v1/sse
    oauth:
      providers:
        - provider: asana
          secret: asana.personal_access_token
          env: ASANA_PERSONAL_ACCESS_TOKEN`,
			expectedType: workingset.ServerTypeRemote,
			validateServer: func(t *testing.T, server *catalog.Server) {
				t.Helper()
				assert.Equal(t, "streamable-http", server.Remote.Transport)
				assert.Equal(t, "https://asana.com/api/mcp/v1/sse", server.Remote.URL)
				require.NotNil(t, server.OAuth)
				assert.Len(t, server.OAuth.Providers, 1)
				assert.Equal(t, "asana", server.OAuth.Providers[0].Provider)
				assert.Equal(t, "asana.personal_access_token", server.OAuth.Providers[0].Secret)
				assert.Equal(t, "ASANA_PERSONAL_ACCESS_TOKEN", server.OAuth.Providers[0].Env)
			},
		},
		{
			name: "remote with dynamic tools",
			serverYAML: `    title: "Cloudflare Audit Logs"
    type: remote
    dynamic:
      tools: true
    remote:
      transport_type: sse
      url: https://auditlogs.mcp.cloudflare.com/sse
    oauth:
      providers:
        - provider: cloudflare-audit-logs
          secret: cloudflare-audit-logs.personal_access_token
          env: CLOUDFLARE_PERSONAL_ACCESS_TOKEN`,
			expectedType: workingset.ServerTypeRemote,
			validateServer: func(t *testing.T, server *catalog.Server) {
				t.Helper()
				assert.Equal(t, "sse", server.Remote.Transport)
				assert.Equal(t, "https://auditlogs.mcp.cloudflare.com/sse", server.Remote.URL)
				require.NotNil(t, server.OAuth)
				assert.Len(t, server.OAuth.Providers, 1)
				assert.Equal(t, "cloudflare-audit-logs", server.OAuth.Providers[0].Provider)
			},
		},
		{
			name: "remote with static tools list (no headers/secrets)",
			serverYAML: `    title: "GitMCP"
    type: remote
    remote:
      transport_type: streamable-http
      url: https://gitmcp.io/docs
    tools:
      - name: match_common_libs_owner_repo_mapping
      - name: fetch_generic_documentation
      - name: search_generic_documentation`,
			expectedType: workingset.ServerTypeRemote,
			validateServer: func(t *testing.T, server *catalog.Server) {
				t.Helper()
				assert.Equal(t, "streamable-http", server.Remote.Transport)
				assert.Equal(t, "https://gitmcp.io/docs", server.Remote.URL)
				assert.Empty(t, server.Remote.Headers)
				assert.Empty(t, server.Secrets)
				assert.Nil(t, server.OAuth)
				assert.Len(t, server.Tools, 3)
				assert.Equal(t, "match_common_libs_owner_repo_mapping", server.Tools[0].Name)
			},
		},
		{
			name: "remote with SSE, headers, and secrets",
			serverYAML: `    title: "Dodo Payments"
    type: remote
    remote:
      transport_type: sse
      url: https://mcp.dodopayments.com/sse
      headers:
        Authorization: "Bearer ${DODO_PAYMENTS_API_KEY}"
    secrets:
      - name: dodo-payments.api_key
        env: DODO_PAYMENTS_API_KEY
        example: <YOUR_API_KEY>`,
			expectedType: workingset.ServerTypeRemote,
			validateServer: func(t *testing.T, server *catalog.Server) {
				t.Helper()
				assert.Equal(t, "sse", server.Remote.Transport)
				assert.Equal(t, "https://mcp.dodopayments.com/sse", server.Remote.URL)
				assert.Equal(t, "Bearer ${DODO_PAYMENTS_API_KEY}", server.Remote.Headers["Authorization"])
				assert.Len(t, server.Secrets, 1)
				assert.Equal(t, "dodo-payments.api_key", server.Secrets[0].Name)
				assert.Equal(t, "DODO_PAYMENTS_API_KEY", server.Secrets[0].Env)
			},
		},
		{
			name: "remote documentation server (no auth)",
			serverYAML: `    title: "Cloudflare Docs"
    type: remote
    remote:
      transport_type: sse
      url: https://docs.mcp.cloudflare.com/sse
    tools:
      - name: search_cloudflare_documentation
      - name: migrate_pages_to_workers_guide`,
			expectedType: workingset.ServerTypeRemote,
			validateServer: func(t *testing.T, server *catalog.Server) {
				t.Helper()
				assert.Equal(t, "sse", server.Remote.Transport)
				assert.Equal(t, "https://docs.mcp.cloudflare.com/sse", server.Remote.URL)
				assert.Empty(t, server.Remote.Headers)
				assert.Empty(t, server.Secrets)
				assert.Nil(t, server.OAuth)
				assert.Len(t, server.Tools, 2)
				assert.Equal(t, "search_cloudflare_documentation", server.Tools[0].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dao := setupTestDB(t)
			ctx := t.Context()

			// Create a temporary legacy catalog file
			tempDir := t.TempDir()
			catalogFile := filepath.Join(tempDir, "test-catalog.yaml")

			legacyCatalogYAML := "name: test-catalog\nregistry:\n  test-server:\n" + tt.serverYAML + "\n"

			err := os.WriteFile(catalogFile, []byte(legacyCatalogYAML), 0o644)
			require.NoError(t, err)

			// Create catalog from legacy catalog
			output := captureStdout(t, func() {
				err := Create(ctx, dao, "test/imported:latest", "", catalogFile, "Imported Catalog")
				require.NoError(t, err)
			})

			assert.Contains(t, output, "Catalog test/imported:latest created")

			// Verify the catalog was created
			catalogs, err := dao.ListCatalogs(ctx)
			require.NoError(t, err)
			assert.Len(t, catalogs, 1)

			catalog := NewFromDb(&catalogs[0])
			assert.Equal(t, "Imported Catalog", catalog.Title)
			assert.Equal(t, "legacy-catalog:test-catalog", catalog.Source)
			require.Len(t, catalog.Servers, 1)

			// Verify server basic properties
			server := catalog.Servers[0]
			assert.Equal(t, tt.expectedType, server.Type)
			require.NotNil(t, server.Snapshot)
			assert.Equal(t, "test-server", server.Snapshot.Server.Name)

			// Verify endpoint is set for remote servers
			if server.Type == workingset.ServerTypeRemote {
				assert.NotEmpty(t, server.Endpoint, "Endpoint should be set for remote servers")
				assert.Equal(t, server.Snapshot.Server.Remote.URL, server.Endpoint, "Endpoint should match the Remote.URL from snapshot")
			}

			// Run custom validation
			tt.validateServer(t, &server.Snapshot.Server)
		})
	}
}
