package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	WorkingSet   string
	dynamicTools bool
	ociService   oci.Service
	docker       docker.Client
}

func NewWorkingSetConfiguration(workingSet string, dynamicTools bool, ociService oci.Service, docker docker.Client) *WorkingSetConfiguration {
	return &WorkingSetConfiguration{
		WorkingSet:   workingSet,
		dynamicTools: dynamicTools,
		ociService:   ociService,
		docker:       docker,
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

	dbWorkingSet, err := dao.GetWorkingSet(ctx, c.WorkingSet)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Special case for the default profile, we're okay with it not existing
			if c.WorkingSet == "default" {
				log.Log("  - Default profile not found, using empty configuration")
				return c.emptyConfiguration()
			}
			return Configuration{}, fmt.Errorf("profile %s not found", c.WorkingSet)
		}
		return Configuration{}, fmt.Errorf("failed to get profile: %w", err)
	}

	workingSet := workingset.NewFromDb(dbWorkingSet)

	if err := workingSet.EnsureSnapshotsResolved(ctx, c.ociService); err != nil {
		return Configuration{}, fmt.Errorf("failed to resolve snapshots: %w", err)
	}

	cfg := make(map[string]map[string]any)
	flattenedSecrets := make(map[string]string)

	providerSecrets, err := c.readSecrets(ctx, workingSet)
	if err != nil {
		return Configuration{}, fmt.Errorf("failed to read secrets: %w", err)
	}

	for provider, s := range providerSecrets {
		for name, value := range s {
			flattenedSecrets[provider+"_"+name] = value
		}
	}

	toolsConfig := c.readTools(workingSet)

	// TODO(cody): Finish making the gateway fully compatible with working sets
	serverNames := make([]string, 0)
	servers := make(map[string]catalog.Server)

	// Load the default catalog to populate servers for dynamic tools
	if c.dynamicTools {
		defaultCatalog, err := dao.GetCatalog(ctx, "mcp/docker-mcp-catalog:latest")
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				log.Log("  - Default catalog not found, dynamic tools will be limited to profile servers. Run `docker mcp catalog-next pull mcp/docker-mcp-catalog:latest` and restart the gateway to add Docker MCP catalog servers to dynamic tools.")
			} else {
				return Configuration{}, fmt.Errorf("failed to get default catalog: %w", err)
			}
		} else {
			log.Log(fmt.Sprintf("  - Read catalog 'mcp/docker-mcp-catalog:latest' for dynamic tools with %d servers", len(defaultCatalog.Servers)))
			for _, server := range defaultCatalog.Servers {
				if server.Snapshot != nil { // should always be true
					servers[server.Snapshot.Server.Name] = server.Snapshot.Server
				}
			}
		}
	}

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
		secrets:     flattenedSecrets,
	}, nil
}

func (c *WorkingSetConfiguration) emptyConfiguration() (Configuration, error) {
	return Configuration{
		serverNames: []string{},
		servers:     make(map[string]catalog.Server),
		config:      make(map[string]map[string]any),
		tools: config.ToolsConfig{
			ServerTools: make(map[string][]string),
		},
		secrets: make(map[string]string),
	}, nil
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

func (c *WorkingSetConfiguration) readSecrets(ctx context.Context, workingSet workingset.WorkingSet) (map[string]map[string]string, error) {
	providerSecrets := make(map[string]map[string]string)
	for providerRef, secretConfig := range workingSet.Secrets {
		servers := getServersUsingProvider(workingSet, providerRef)

		switch secretConfig.Provider {
		case workingset.SecretProviderDockerDesktop:
			secrets, err := c.readDockerDesktopSecrets(ctx, servers)
			if err != nil {
				return nil, fmt.Errorf("failed to read docker desktop secrets: %w", err)
			}
			providerSecrets[providerRef] = secrets
		default:
			return nil, fmt.Errorf("unknown secret provider: %s", secretConfig.Provider)
		}
	}

	return providerSecrets, nil
}

func (c *WorkingSetConfiguration) readDockerDesktopSecrets(ctx context.Context, servers []workingset.Server) (map[string]string, error) {
	// Use a map to deduplicate secret names
	uniqueSecretNames := make(map[string]struct{})

	for _, server := range servers {
		serverSpec := server.Snapshot.Server

		for _, s := range serverSpec.Secrets {
			uniqueSecretNames[s.Name] = struct{}{}
		}
	}

	if len(uniqueSecretNames) == 0 {
		return map[string]string{}, nil
	}

	// Convert map keys to slice
	var secretNames []string
	for name := range uniqueSecretNames {
		secretNames = append(secretNames, name)
	}

	log.Log("  - Reading secrets from Docker Desktop", secretNames)
	secretsByName, err := c.docker.ReadSecrets(ctx, secretNames, true)
	if err != nil {
		return nil, fmt.Errorf("finding secrets %s: %w", secretNames, err)
	}

	return secretsByName, nil
}

func getServersUsingProvider(workingSet workingset.WorkingSet, providerRef string) []workingset.Server {
	servers := make([]workingset.Server, 0)
	for _, server := range workingSet.Servers {
		if server.Secrets == providerRef {
			servers = append(servers, server)
		}
	}
	return servers
}
