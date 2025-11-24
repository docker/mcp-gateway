package migrate

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	legacycatalog "github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/docker"
)

// setupTestEnvironment creates a temporary directory structure with legacy config files
func setupTestEnvironment(t *testing.T) string {
	t.Helper()

	// Save original HOME env var
	originalHome := os.Getenv("HOME")

	// Create temp directory
	tempDir := t.TempDir()

	// Override HOME to point to temp directory
	os.Setenv("HOME", tempDir)

	// Create .docker/mcp directory structure
	mcpDir := filepath.Join(tempDir, ".docker", "mcp")
	catalogsDir := filepath.Join(mcpDir, "catalogs")
	err := os.MkdirAll(catalogsDir, 0o755)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.Setenv("HOME", originalHome)
	})

	return mcpDir
}

// setupTestDB creates a test database
func setupTestDB(t *testing.T) db.DAO {
	t.Helper()

	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	dao, err := db.New(db.WithDatabaseFile(dbFile))
	require.NoError(t, err)

	return dao
}

func TestMigrateConfig_SkipsIfAlreadyRun(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create legacy files (these should not be read)
	writeTestLegacyFiles(t, mcpDir, "server1")

	// Mark migration as already completed
	err := dao.UpdateMigrationStatus(ctx, db.MigrationStatus{
		Status: MigrationStatusSuccess,
		Logs:   "Previously migrated",
	})
	require.NoError(t, err)

	// Run migration - should skip
	var mockDocker docker.Client = &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify no working sets were created
	workingSets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Empty(t, workingSets, "No working sets should be created when migration already ran")

	// Verify status unchanged
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusSuccess, status.Status)
	assert.Equal(t, "Previously migrated", status.Logs)
}

func TestMigrateConfig_SuccessWithServers(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create legacy files with servers
	writeTestLegacyFiles(t, mcpDir, "server1", "server2")

	mockDocker := &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify migration status is success
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusSuccess, status.Status)
	assert.Contains(t, status.Logs, "default profile created with 2 servers")

	// Verify working set was created
	workingSets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Len(t, workingSets, 1)
	assert.Equal(t, "default", workingSets[0].ID)
	assert.Equal(t, "Default Profile", workingSets[0].Name)

	// Verify servers in working set
	assert.Len(t, workingSets[0].Servers, 2)

	// Verify secrets
	assert.Len(t, workingSets[0].Secrets, 1)
	assert.Equal(t, "docker-desktop-store", workingSets[0].Secrets["default"].Provider)

	// Verify legacy files were backed up
	assertLegacyFilesBackedUp(t, mcpDir)
}

func TestMigrateConfig_SuccessWithSingleServer(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create legacy files with one server
	writeTestLegacyFiles(t, mcpDir, "postgres-server")

	mockDocker := &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify migration status is success
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusSuccess, status.Status)
	assert.Contains(t, status.Logs, "default profile created with 1 servers")

	// Verify working set was created
	workingSets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Len(t, workingSets, 1)
	assert.Len(t, workingSets[0].Servers, 1)

	// Verify server details
	server := workingSets[0].Servers[0]
	assert.Equal(t, "image", server.Type)
	assert.Nil(t, server.Tools)
	assert.Nil(t, server.Config)
	assert.Equal(t, "test/postgres-server:latest", server.Image)
	assert.Equal(t, "default", server.Secrets)
	assert.NotNil(t, server.Snapshot)
	assert.Equal(t, "postgres-server", server.Snapshot.Server.Name)
}

func TestMigrateConfig_SkipsWithNoServers(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create legacy files with NO servers
	writeTestLegacyFiles(t, mcpDir)

	mockDocker := &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify migration status is success but no profile created
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusSuccess, status.Status)
	assert.Contains(t, status.Logs, "no existing installed servers found, skipping default profile creation")

	// Verify NO working set was created
	workingSets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Empty(t, workingSets)

	// Verify legacy files were still backed up
	assertLegacyFilesBackedUp(t, mcpDir)
}

func TestMigrateConfig_FailureReadingLegacyFiles(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create malformed registry.yaml
	registryPath := filepath.Join(mcpDir, "registry.yaml")
	err := os.WriteFile(registryPath, []byte("invalid: yaml: [[["), 0o644)
	require.NoError(t, err)

	mockDocker := &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify migration status is failed
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusFailed, status.Status)
	assert.Contains(t, status.Logs, "failed to read legacy defaults")

	// Verify NO working set was created
	workingSets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Empty(t, workingSets)
}

func TestMigrateConfig_FailureMissingRegistryFile(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create config.yaml and tools.yaml but NOT registry.yaml
	configPath := filepath.Join(mcpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte("{}"), 0o644)
	require.NoError(t, err)

	toolsPath := filepath.Join(mcpDir, "tools.yaml")
	err = os.WriteFile(toolsPath, []byte("{}"), 0o644)
	require.NoError(t, err)

	// Don't create registry.yaml or catalog

	mockDocker := &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify migration status is failed
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusFailed, status.Status)
	assert.Contains(t, status.Logs, "failed to read legacy defaults")
}

func TestMigrateConfig_WithServerConfigAndTools(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create legacy files with servers that have config and tools
	serverNames := []string{"server1"}

	// Registry
	registryYaml := `registry:
  server1:
    ref: ""`
	err := os.WriteFile(filepath.Join(mcpDir, "registry.yaml"), []byte(registryYaml), 0o644)
	require.NoError(t, err)

	// Config
	configYaml := `server1:
  timeout: 30
  retries: 3`
	err = os.WriteFile(filepath.Join(mcpDir, "config.yaml"), []byte(configYaml), 0o644)
	require.NoError(t, err)

	// Tools
	toolsYaml := `server1:
  - tool1
  - tool2
  - tool3`
	err = os.WriteFile(filepath.Join(mcpDir, "tools.yaml"), []byte(toolsYaml), 0o644)
	require.NoError(t, err)

	// Catalog
	writeCatalogFile(t, mcpDir, serverNames)

	mockDocker := &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify migration status is success
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusSuccess, status.Status)

	// Verify working set
	workingSets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	require.Len(t, workingSets, 1)
	require.Len(t, workingSets[0].Servers, 1)

	server := workingSets[0].Servers[0]

	// Verify config
	assert.NotNil(t, server.Config)
	assert.InEpsilon(t, float64(30), server.Config["timeout"], 0.0000001)
	assert.InEpsilon(t, float64(3), server.Config["retries"], 0.0000001)

	// Verify tools
	assert.Equal(t, []string{"tool1", "tool2", "tool3"}, server.Tools)
}

func TestMigrateConfig_SkipsNonServerTypes(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create registry with mixed server types
	registryYaml := `registry:
  good-server:
    ref: ""
  bad-server:
    ref: ""`
	err := os.WriteFile(filepath.Join(mcpDir, "registry.yaml"), []byte(registryYaml), 0o644)
	require.NoError(t, err)

	// Config
	err = os.WriteFile(filepath.Join(mcpDir, "config.yaml"), []byte("{}"), 0o644)
	require.NoError(t, err)

	// Tools
	err = os.WriteFile(filepath.Join(mcpDir, "tools.yaml"), []byte("{}"), 0o644)
	require.NoError(t, err)

	// Catalog with one valid and one invalid server type
	catalogYaml := `registry:
  good-server:
    type: server
    image: test/good:latest
    title: Good Server
    description: A good server
  bad-server:
    type: bad
    image: test/bad:latest
    title: Bad Server
    description: A bad server (not supported)`
	catalogsDir := filepath.Join(mcpDir, "catalogs")
	err = os.MkdirAll(catalogsDir, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(catalogsDir, legacycatalog.DockerCatalogFilename), []byte(catalogYaml), 0o644)
	require.NoError(t, err)

	mockDocker := &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify migration status is success
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusSuccess, status.Status)
	assert.Contains(t, status.Logs, "server bad-server has an invalid server type: bad, skipping")

	// Verify only good-server was added
	workingSets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	require.Len(t, workingSets, 1)
	assert.Len(t, workingSets[0].Servers, 1)
	assert.Equal(t, "test/good:latest", workingSets[0].Servers[0].Image)
}

func TestMigrateConfig_ServerNotInCatalog(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create registry with server
	registryYaml := `registry:
  missing-server:
    ref: ""`
	err := os.WriteFile(filepath.Join(mcpDir, "registry.yaml"), []byte(registryYaml), 0o644)
	require.NoError(t, err)

	// Config
	err = os.WriteFile(filepath.Join(mcpDir, "config.yaml"), []byte("{}"), 0o644)
	require.NoError(t, err)

	// Tools
	err = os.WriteFile(filepath.Join(mcpDir, "tools.yaml"), []byte("{}"), 0o644)
	require.NoError(t, err)

	// Empty catalog (server not in it)
	catalogYaml := `registry: {}`
	catalogsDir := filepath.Join(mcpDir, "catalogs")
	err = os.MkdirAll(catalogsDir, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(catalogsDir, legacycatalog.DockerCatalogFilename), []byte(catalogYaml), 0o644)
	require.NoError(t, err)

	mockDocker := &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify migration status is success
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusSuccess, status.Status)
	assert.Contains(t, status.Logs, "missing-server not found in old catalog, skipping")

	// Verify no servers were added, but profile might still be created if there are other servers
	// In this case, with only one server that's missing, no profile should be created
	workingSets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	assert.Len(t, workingSets, 1)
	assert.Empty(t, workingSets[0].Servers)
}

func TestMigrateConfig_MultipleServersWithPartialFailure(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create registry with multiple servers
	registryYaml := `registry:
  good-server1:
    ref: ""
  missing-server:
    ref: ""
  good-server2:
    ref: ""`
	err := os.WriteFile(filepath.Join(mcpDir, "registry.yaml"), []byte(registryYaml), 0o644)
	require.NoError(t, err)

	// Config
	err = os.WriteFile(filepath.Join(mcpDir, "config.yaml"), []byte("{}"), 0o644)
	require.NoError(t, err)

	// Tools
	err = os.WriteFile(filepath.Join(mcpDir, "tools.yaml"), []byte("{}"), 0o644)
	require.NoError(t, err)

	// Catalog with only 2 of 3 servers
	catalogYaml := `registry:
  good-server1:
    type: server
    image: test/good1:latest
    title: Good Server 1
  good-server2:
    type: server
    image: test/good2:latest
    title: Good Server 2`
	catalogsDir := filepath.Join(mcpDir, "catalogs")
	err = os.MkdirAll(catalogsDir, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(catalogsDir, legacycatalog.DockerCatalogFilename), []byte(catalogYaml), 0o644)
	require.NoError(t, err)

	mockDocker := &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify migration status is success (partial success)
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusSuccess, status.Status)
	assert.Contains(t, status.Logs, "missing-server not found in old catalog, skipping")
	assert.Contains(t, status.Logs, "added server good-server1 to profile")
	assert.Contains(t, status.Logs, "added server good-server2 to profile")

	// Verify two servers were added
	workingSets, err := dao.ListWorkingSets(ctx)
	require.NoError(t, err)
	require.Len(t, workingSets, 1)
	assert.Len(t, workingSets[0].Servers, 2)
}

func TestMigrateConfig_LegacyFilesBackedUpOnSuccess(t *testing.T) {
	mcpDir := setupTestEnvironment(t)

	dao := setupTestDB(t)
	ctx := t.Context()

	// Create legacy files
	writeTestLegacyFiles(t, mcpDir, "server1")

	// Also create catalog.json
	catalogIndexPath := filepath.Join(mcpDir, "catalog.json")
	err := os.WriteFile(catalogIndexPath, []byte(`{"catalogs":{}}`), 0o644)
	require.NoError(t, err)

	mockDocker := &mockDockerClient{}
	MigrateConfig(ctx, mockDocker, dao)

	// Verify migration status is success
	status, err := dao.GetMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, MigrationStatusSuccess, status.Status)

	// Verify all legacy files were backed up
	assertLegacyFilesBackedUp(t, mcpDir)

	// Verify catalog.json was also backed up
	backupCatalogIndexPath := filepath.Join(mcpDir, ".backup", "catalog.json")
	_, err = os.Stat(backupCatalogIndexPath)
	assert.False(t, os.IsNotExist(err), "catalog.json should be backed up")

	// Verify original catalog.json no longer exists
	_, err = os.Stat(catalogIndexPath)
	assert.True(t, os.IsNotExist(err), "original catalog.json should be removed")
}

// Helper functions

func writeTestLegacyFiles(t *testing.T, mcpDir string, serverNames ...string) {
	t.Helper()

	// Build registry YAML
	registryYaml := "registry:\n"
	for _, serverName := range serverNames {
		registryYaml += "  " + serverName + ":\n"
		registryYaml += "    ref: \"\"\n"
	}

	err := os.WriteFile(filepath.Join(mcpDir, "registry.yaml"), []byte(registryYaml), 0o644)
	require.NoError(t, err)

	// Create empty config.yaml
	err = os.WriteFile(filepath.Join(mcpDir, "config.yaml"), []byte("{}"), 0o644)
	require.NoError(t, err)

	// Create empty tools.yaml
	err = os.WriteFile(filepath.Join(mcpDir, "tools.yaml"), []byte("{}"), 0o644)
	require.NoError(t, err)

	// Create catalog file
	writeCatalogFile(t, mcpDir, serverNames)
}

func writeCatalogFile(t *testing.T, mcpDir string, serverNames []string) {
	t.Helper()

	catalogYaml := "registry:\n"
	for _, serverName := range serverNames {
		catalogYaml += "  " + serverName + ":\n"
		catalogYaml += "    type: server\n"
		catalogYaml += "    image: test/" + serverName + ":latest\n"
		catalogYaml += "    title: " + serverName + "\n"
		catalogYaml += "    description: Test server " + serverName + "\n"
	}

	catalogsDir := filepath.Join(mcpDir, "catalogs")
	err := os.MkdirAll(catalogsDir, 0o755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(catalogsDir, legacycatalog.DockerCatalogFilename), []byte(catalogYaml), 0o644)
	require.NoError(t, err)
}

func assertLegacyFilesBackedUp(t *testing.T, mcpDir string) {
	t.Helper()

	backupDir := filepath.Join(mcpDir, ".backup")

	// Verify backup directory exists
	_, err := os.Stat(backupDir)
	assert.False(t, os.IsNotExist(err), "backup directory should exist")

	// Verify original files no longer exist in original location
	registryPath := filepath.Join(mcpDir, "registry.yaml")
	_, err = os.Stat(registryPath)
	assert.True(t, os.IsNotExist(err), "original registry.yaml should be removed")

	configPath := filepath.Join(mcpDir, "config.yaml")
	_, err = os.Stat(configPath)
	assert.True(t, os.IsNotExist(err), "original config.yaml should be removed")

	toolsPath := filepath.Join(mcpDir, "tools.yaml")
	_, err = os.Stat(toolsPath)
	assert.True(t, os.IsNotExist(err), "original tools.yaml should be removed")

	catalogPath := filepath.Join(mcpDir, "catalogs", legacycatalog.DockerCatalogFilename)
	_, err = os.Stat(catalogPath)
	assert.True(t, os.IsNotExist(err), "original catalog file should be removed")

	// Verify files exist in backup directory
	backupRegistryPath := filepath.Join(backupDir, "registry.yaml")
	_, err = os.Stat(backupRegistryPath)
	assert.False(t, os.IsNotExist(err), "registry.yaml should be backed up")

	backupConfigPath := filepath.Join(backupDir, "config.yaml")
	_, err = os.Stat(backupConfigPath)
	assert.False(t, os.IsNotExist(err), "config.yaml should be backed up")

	backupToolsPath := filepath.Join(backupDir, "tools.yaml")
	_, err = os.Stat(backupToolsPath)
	assert.False(t, os.IsNotExist(err), "tools.yaml should be backed up")

	backupCatalogPath := filepath.Join(backupDir, legacycatalog.DockerCatalogFilename)
	_, err = os.Stat(backupCatalogPath)
	assert.False(t, os.IsNotExist(err), "catalog file should be backed up")
}

// mockDockerClient is a simple mock implementation of docker.Client for testing
type mockDockerClient struct {
	inspectVolumeFunc func(ctx context.Context, name string) (volume.Volume, error)
}

func (m *mockDockerClient) ContainerExists(_ context.Context, _ string) (bool, container.InspectResponse, error) {
	return false, container.InspectResponse{}, nil
}

func (m *mockDockerClient) RemoveContainer(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *mockDockerClient) StartContainer(_ context.Context, _ string, _ container.Config, _ container.HostConfig, _ network.NetworkingConfig) error {
	return nil
}

func (m *mockDockerClient) StopContainer(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockDockerClient) FindContainerByLabel(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockDockerClient) FindAllContainersByLabel(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (m *mockDockerClient) InspectContainer(_ context.Context, _ string) (container.InspectResponse, error) {
	return container.InspectResponse{}, nil
}

func (m *mockDockerClient) ReadLogs(_ context.Context, _ string, _ container.LogsOptions) (io.ReadCloser, error) {
	return nil, nil //nolint:nilnil
}

func (m *mockDockerClient) ImageExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockDockerClient) InspectImage(_ context.Context, _ string) (image.InspectResponse, error) {
	return image.InspectResponse{}, nil
}

func (m *mockDockerClient) PullImage(_ context.Context, _ string) error {
	return nil
}

func (m *mockDockerClient) PullImages(_ context.Context, _ ...string) error {
	return nil
}

func (m *mockDockerClient) CreateNetwork(_ context.Context, _ string, _ bool, _ map[string]string) error {
	return nil
}

func (m *mockDockerClient) RemoveNetwork(_ context.Context, _ string) error {
	return nil
}

func (m *mockDockerClient) ConnectNetwork(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (m *mockDockerClient) InspectVolume(ctx context.Context, name string) (volume.Volume, error) {
	if m.inspectVolumeFunc != nil {
		return m.inspectVolumeFunc(ctx, name)
	}
	// Return error to simulate volume not found
	return volume.Volume{}, sql.ErrNoRows
}

func (m *mockDockerClient) ReadSecrets(_ context.Context, _ []string, _ bool) (map[string]string, error) {
	return nil, nil //nolint:nilnil
}
