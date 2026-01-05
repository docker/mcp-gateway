package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/migrate"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

type WorkingSetConfiguration struct {
	config     Config
	ociService oci.Service
	docker     docker.Client
}

func NewWorkingSetConfiguration(config Config, ociService oci.Service, docker docker.Client) *WorkingSetConfiguration {
	return &WorkingSetConfiguration{
		config:     config,
		ociService: ociService,
		docker:     docker,
	}
}

func (c *WorkingSetConfiguration) Read(ctx context.Context) (Configuration, chan Configuration, func() error, error) {
	dao, err := db.New()
	if err != nil {
		return Configuration{}, nil, nil, fmt.Errorf("failed to create database client: %w", err)
	}

	// Do migration from legacy files
	migrate.MigrateConfig(ctx, c.docker, dao)

	configuration, err := c.readOnce(ctx, dao)
	if err != nil {
		return Configuration{}, nil, nil, err
	}

	// TODO(cody): Stub for now
	updates := make(chan Configuration)

	return configuration, updates, func() error { return nil }, nil
}

func (c *WorkingSetConfiguration) readOnce(ctx context.Context, dao db.DAO) (Configuration, error) {
	start := time.Now()
	log.Log("- Reading profile configuration...")

	dbWorkingSet, err := dao.GetWorkingSet(ctx, c.config.WorkingSet)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Special case for the default profile, we're okay with it not existing
			if c.config.WorkingSet == "default" {
				log.Log("  - Default profile not found, using empty configuration")
				return c.emptyConfiguration(ctx, dao)
			}
			return Configuration{}, fmt.Errorf("profile %s not found", c.config.WorkingSet)
		}
		return Configuration{}, fmt.Errorf("failed to get profile: %w", err)
	}

	workingSet := workingset.NewFromDb(dbWorkingSet)

	if err := workingSet.EnsureSnapshotsResolved(ctx, c.ociService); err != nil {
		return Configuration{}, fmt.Errorf("failed to resolve snapshots: %w", err)
	}

	cfg := make(map[string]map[string]any)

	// Build se:// URIs for secrets (resolved at runtime by secrets engine)
	// Keys are prefixed with the secrets provider reference to namespace them
	secrets := make(map[string]string)
	for _, server := range workingSet.Servers {
		providerPrefix := ""
		if server.Secrets != "" {
			providerPrefix = server.Secrets + "_"
		}
		for _, s := range server.Snapshot.Server.Secrets {
			secrets[providerPrefix+s.Name] = fmt.Sprintf("se://%s", secret.GetSecretKey(s.Name))
		}
	}

	toolsConfig := c.readTools(workingSet)

	// TODO(cody): Finish making the gateway fully compatible with working sets
	serverNames := make([]string, 0)
	servers := make(map[string]catalog.Server)

	// Load all catalogs to populate servers for dynamic tools
	allCatalogServers, err := c.readAllCatalogServers(ctx, dao)
	if err != nil {
		return Configuration{}, fmt.Errorf("failed to read all catalog servers: %w", err)
	}
	maps.Copy(servers, allCatalogServers)

	for _, server := range workingSet.Servers {
		// Skip registry servers for now
		if server.Type != workingset.ServerTypeImage && server.Type != workingset.ServerTypeRemote {
			continue
		}

		serverName := server.Snapshot.Server.Name

		servers[serverName] = server.Snapshot.Server
		serverNames = append(serverNames, serverName)

		cfg[serverName] = server.Config

		// TODO(cody): temporary hack to namespace secrets to provider
		if server.Secrets != "" {
			for i := range server.Snapshot.Server.Secrets {
				server.Snapshot.Server.Secrets[i].Name = server.Secrets + "_" + server.Snapshot.Server.Secrets[i].Name
			}
		}
	}

	log.Log("- Configuration read in", time.Since(start))

	return Configuration{
		serverNames: serverNames,
		servers:     servers,
		config:      cfg,
		tools:       toolsConfig,
		secrets:     secrets,
	}, nil
}

func (c *WorkingSetConfiguration) emptyConfiguration(ctx context.Context, dao db.DAO) (Configuration, error) {
	// Load all catalogs to populate servers for dynamic tools
	allCatalogServers, err := c.readAllCatalogServers(ctx, dao)
	if err != nil {
		return Configuration{}, fmt.Errorf("failed to read all catalog servers: %w", err)
	}

	return Configuration{
		serverNames: []string{},
		servers:     allCatalogServers,
		config:      make(map[string]map[string]any),
		tools: config.ToolsConfig{
			ServerTools: make(map[string][]string),
		},
		secrets: make(map[string]string),
	}, nil
}

func (c *WorkingSetConfiguration) readAllCatalogServers(ctx context.Context, dao db.DAO) (map[string]catalog.Server, error) {
	servers := make(map[string]catalog.Server)
	if c.config.DynamicTools {
		allCatalogs, err := dao.ListCatalogs(ctx)
		if err != nil {
			return servers, fmt.Errorf("failed to list catalogs: %w", err)
		}

		if len(allCatalogs) == 0 {
			log.Log("  - No catalogs found, dynamic tools will be limited to profile servers. Run `docker mcp catalog-next pull mcp/docker-mcp-catalog:latest` and restart the gateway to add Docker MCP catalog servers to dynamic tools.")
		} else {
			log.Log(fmt.Sprintf("  - Loading %d catalog(s) for dynamic tools", len(allCatalogs)))
			for _, cat := range allCatalogs {
				log.Log(fmt.Sprintf("    - Processing catalog '%s' with %d servers", cat.Ref, len(cat.Servers)))
				for _, server := range cat.Servers {
					if server.Snapshot != nil { // should always be true
						servers[server.Snapshot.Server.Name] = server.Snapshot.Server
					}
				}
			}
			log.Log(fmt.Sprintf("  - Total servers loaded from all catalogs: %d", len(servers)))
		}
	}
	return servers, nil
}

func (c *WorkingSetConfiguration) readTools(workingSet workingset.WorkingSet) config.ToolsConfig {
	toolsConfig := config.ToolsConfig{
		ServerTools: make(map[string][]string),
	}
	for _, server := range workingSet.Servers {
		if server.Tools == nil {
			continue
		}
		if _, exists := toolsConfig.ServerTools[server.Snapshot.Server.Name]; exists {
			log.Log(fmt.Sprintf("Warning: overlapping server tools '%s' found in profile, overwriting previous value", server.Snapshot.Server.Name))
		}
		toolsConfig.ServerTools[server.Snapshot.Server.Name] = server.Tools
	}
	return toolsConfig
}
