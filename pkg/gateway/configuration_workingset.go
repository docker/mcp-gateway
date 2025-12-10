package gateway

import (
	"context"
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
	workingSetID   string
	workingSetFile string
	ociService     oci.Service
	docker         docker.Client
}

func NewWorkingSetConfiguration(workingSetID string, workingSetFile string, ociService oci.Service, docker docker.Client) *WorkingSetConfiguration {
	return &WorkingSetConfiguration{
		workingSetID:   workingSetID,
		workingSetFile: workingSetFile,
		ociService:     ociService,
		docker:         docker,
	}
}

func (c *WorkingSetConfiguration) Read(ctx context.Context) (Configuration, chan Configuration, func() error, error) {
	var loader WorkingSetLoader
	cleanup := func() error { return nil }

	if c.workingSetID != "" {
		dao, err := db.New()
		if err != nil {
			return Configuration{}, nil, nil, fmt.Errorf("failed to create database client: %w", err)
		}
		cleanup = func() error { return dao.Close() }

		// Do migration from legacy files
		migrate.MigrateConfig(ctx, c.docker, dao)

		loader = NewWorkingSetDatabaseLoader(c.workingSetID, dao)
	} else if c.workingSetFile != "" {
		loader = NewWorkingSetFileLoader(c.workingSetFile, c.ociService)
	} else {
		return Configuration{}, nil, nil, fmt.Errorf("no working set ID or file provided")
	}

	configuration, err := c.readOnce(ctx, loader)
	if err != nil {
		return Configuration{}, nil, nil, err
	}

	// TODO(cody): Stub for now
	updates := make(chan Configuration)

	return configuration, updates, cleanup, nil
}

func (c *WorkingSetConfiguration) readOnce(ctx context.Context, loader WorkingSetLoader) (Configuration, error) {
	start := time.Now()
	log.Log("- Reading profile configuration...")

	workingSet, err := loader.ReadWorkingSet(ctx)
	if err != nil {
		if errors.Is(err, ErrWorkingSetNotFound) {
			return Configuration{}, err
		}
		return Configuration{}, fmt.Errorf("failed to get profile: %w", err)
	}

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
	for _, server := range workingSet.Servers {
		// Skip registry servers for now
		if server.Type != workingset.ServerTypeImage && server.Type != workingset.ServerTypeRemote {
			continue
		}

		serverName := server.Snapshot.Server.Name

		if _, exists := servers[serverName]; exists {
			return Configuration{}, fmt.Errorf("duplicate server names: %s", serverName)
		}

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
