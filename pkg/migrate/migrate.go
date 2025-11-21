package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	legacycatalog "github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

const (
	MigrationStatusSuccess = "success"
	MigrationStatusFailed  = "failed"
)

//revive:disable
func MigrateConfig(ctx context.Context, docker docker.Client, dao db.DAO) {
	_, err := dao.GetMigrationStatus(ctx)
	if err == nil {
		// Migration already run, skip
		return
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		fmt.Fprintf(os.Stderr, "failed to get migration status: %s", err.Error())
		return
	}

	// err == sql.ErrNoRows so we need to perform the migration

	status := MigrationStatusFailed
	logs := []string{}

	defer func() {
		err = dao.UpdateMigrationStatus(ctx, db.MigrationStatus{
			Status: status,
			Logs:   strings.Join(logs, "\n"),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to update migration status: %s", err.Error())
		}
	}()

	// Anything beyond this point should log failures to `logs`

	registry, cfg, tools, oldCatalog, err := readLegacyDefaults(ctx, docker)
	if err != nil {
		logs = append(logs, fmt.Sprintf("failed to read legacy defaults: %s", err.Error()))
		// Failed migration
		return
	}

	// Only create a default profile if there are existing installed servers
	if len(registry.ServerNames()) > 0 {
		createLogs, err := createDefaultProfile(ctx, dao, registry, cfg, tools, oldCatalog)
		if err != nil {
			logs = append(logs, fmt.Sprintf("failed to create default profile: %s", err.Error()))
			// Failed migration
			return
		}
		logs = append(logs, createLogs...)
		logs = append(logs, fmt.Sprintf("default profile created with %d servers", len(registry.ServerNames())))
	} else {
		logs = append(logs, "no existing installed servers found, skipping default profile creation")
	}

	// Migration considered successful by this point
	status = MigrationStatusSuccess

	err = backupLegacyFiles()
	if err != nil {
		logs = append(logs, fmt.Sprintf("failed to backup legacy files: %s", err.Error()))
		return
	}
}

func createDefaultProfile(ctx context.Context, dao db.DAO, registry *config.Registry, cfg map[string]map[string]any, tools *config.ToolsConfig, oldCatalog *legacycatalog.Catalog) ([]string, error) {
	logs := []string{}

	// Add default secrets
	secrets := make(map[string]workingset.Secret)
	secrets["default"] = workingset.Secret{
		Provider: workingset.SecretProviderDockerDesktop,
	}

	profile := workingset.WorkingSet{
		ID:      "default",
		Name:    "Default Profile",
		Version: workingset.CurrentWorkingSetVersion,
		Servers: make([]workingset.Server, 0),
		Secrets: secrets,
	}

	for _, server := range registry.ServerNames() {
		oldServer, ok := oldCatalog.Servers[server]
		if !ok {
			logs = append(logs, fmt.Sprintf("server %s not found in old catalog, skipping", server))
			continue // Ignore
		}
		oldServer.Name = server // Name is set after loading

		profileServer := workingset.Server{
			Config:  cfg[server],
			Tools:   tools.ServerTools[server],
			Secrets: "default",
		}
		if oldServer.Type == "server" {
			profileServer.Type = workingset.ServerTypeImage
			profileServer.Image = oldServer.Image
		} else {
			// TODO(cody): Support remotes
			logs = append(logs, fmt.Sprintf("server %s has an invalid server type: %s, skipping", server, oldServer.Type))
			continue // Ignore
		}
		profileServer.Snapshot = &workingset.ServerSnapshot{
			Server: oldServer,
		}
		profile.Servers = append(profile.Servers, profileServer)
		logs = append(logs, fmt.Sprintf("added server %s to profile", server))
	}

	if err := profile.Validate(); err != nil {
		return logs, fmt.Errorf("invalid profile: %w", err)
	}

	err := dao.CreateWorkingSet(ctx, profile.ToDb())
	if err != nil {
		return logs, fmt.Errorf("failed to create profile: %w", err)
	}

	return logs, nil
}

func readLegacyDefaults(ctx context.Context, docker docker.Client) (*config.Registry, map[string]map[string]any, *config.ToolsConfig, *legacycatalog.Catalog, error) {
	registryPath, err := config.FilePath("registry.yaml")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get registry path: %w", err)
	}
	configPath, err := config.FilePath("config.yaml")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get config path: %w", err)
	}
	toolsPath, err := config.FilePath("tools.yaml")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get tools path: %w", err)
	}

	registryYaml, err := config.ReadConfigFile(ctx, docker, registryPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to read registry file: %w", err)
	}
	registry, err := config.ParseRegistryConfig(registryYaml)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse registry file: %w", err)
	}

	configYaml, err := config.ReadConfigFile(ctx, docker, configPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to read config file: %w", err)
	}
	cfg, err := config.ParseConfig(configYaml)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	toolsYaml, err := config.ReadConfigFile(ctx, docker, toolsPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to read tools file: %w", err)
	}
	tools, err := config.ParseToolsConfig(toolsYaml)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse tools file: %w", err)
	}

	mcpCatalog, err := legacycatalog.ReadFrom(ctx, []string{legacycatalog.DockerCatalogFilename})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("reading catalog: %w", err)
	}

	return &registry, cfg, &tools, &mcpCatalog, nil
}

func backupLegacyFiles() error {
	// Create backup directory
	backupDir, err := config.FilePath(".backup")
	if err != nil {
		return fmt.Errorf("failed to get backup directory path: %w", err)
	}

	err = os.MkdirAll(backupDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Get paths to legacy files
	registryPath, err := config.FilePath("registry.yaml")
	if err != nil {
		return fmt.Errorf("failed to get registry path: %w", err)
	}
	configPath, err := config.FilePath("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	toolsPath, err := config.FilePath("tools.yaml")
	if err != nil {
		return fmt.Errorf("failed to get tools path: %w", err)
	}

	catalogIndexPath, err := config.FilePath("catalog.json")
	if err != nil {
		return fmt.Errorf("failed to get catalog index path: %w", err)
	}

	catalogsDir, err := config.FilePath("catalogs")
	if err != nil {
		return fmt.Errorf("failed to get old catalog path: %w", err)
	}

	oldCatalogPath := filepath.Join(catalogsDir, legacycatalog.DockerCatalogFilename)

	// Move files to backup directory
	_ = moveFile(registryPath, filepath.Join(backupDir, "registry.yaml"))
	_ = moveFile(configPath, filepath.Join(backupDir, "config.yaml"))
	_ = moveFile(toolsPath, filepath.Join(backupDir, "tools.yaml"))
	_ = moveFile(catalogIndexPath, filepath.Join(backupDir, "catalog.json"))
	_ = moveFile(oldCatalogPath, filepath.Join(backupDir, legacycatalog.DockerCatalogFilename))

	// We use os.Remove to remove the directory, so only it's only removed if empty
	// We don't want to remove any custom catalog yamls the user may have added
	_ = os.Remove(catalogsDir)

	return nil
}

// moveFile moves a file from src to dst. If src doesn't exist, it's a no-op.
func moveFile(src, dst string) error {
	// Check if source file exists
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to move
	}

	// Move the file
	return os.Rename(src, dst)
}
