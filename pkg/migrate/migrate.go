package migrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	legacycatalog "github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/utils"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

const (
	MigrationStatusPending = "pending"
	MigrationStatusSuccess = "success"
	MigrationStatusFailed  = "failed"
)

var errMigrationAlreadyRunning = errors.New("migration already running")

//revive:disable
func MigrateConfig(ctx context.Context, docker docker.Client, dao db.DAO) {
	var acquired bool
	// It's possible another cli instance is already running the migration,
	// so retry this 5 times as long as the error is errMigrationAlreadyRunning
	err := utils.RetryIfErrorIs(5, 300*time.Millisecond, func() error {
		migrationStatus, ac, err := dao.TryAcquireMigration(ctx, MigrationStatusPending)
		if err != nil {
			var sqliteErr *sqlite.Error
			// If another "acquire" is in progress and is slow or stuck, it can return SQLITE_BUSY
			if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_BUSY {
				return errMigrationAlreadyRunning
			}
			return err
		}
		acquired = ac

		if migrationStatus.LastUpdated != nil &&
			migrationStatus.Status == MigrationStatusPending &&
			time.Since(*migrationStatus.LastUpdated) > 10*time.Second {
			// Very unlikely, but if stuck in pending state for too long, override the acquired flag.
			// This can happen if the migration gets interrupted. Let's try to finish it.
			acquired = true
			return nil
		}

		if !acquired && migrationStatus.Status == MigrationStatusPending {
			// Pending migration happening in another instance
			return errMigrationAlreadyRunning
		}

		return nil
	}, errMigrationAlreadyRunning)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get migration status: %s\n", err.Error())
		return
	}

	// Didn't acquire the migration, assume already completed
	if !acquired {
		return
	}

	// Otherwise, run the migration

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
		switch oldServer.Type {
		case "server":
			profileServer.Type = workingset.ServerTypeImage
			profileServer.Image = oldServer.Image
		case "remote":
			profileServer.Type = workingset.ServerTypeRemote
			profileServer.Endpoint = oldServer.Remote.URL
		default:
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
