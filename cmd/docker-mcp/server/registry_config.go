package server

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/docker"
)

// loadRegistryWithConfig reads the registry and populates it with user config values.
// It returns the populated registry and the userConfig map for further use.
func loadRegistryWithConfig(ctx context.Context, docker docker.Client) (config.Registry, map[string]map[string]any, error) {
	// Read registry.yaml that contains which servers are enabled.
	registryYAML, err := config.ReadRegistry(ctx, docker)
	if err != nil {
		return config.Registry{}, nil, fmt.Errorf("reading registry config: %w", err)
	}

	registry, err := config.ParseRegistryConfig(registryYAML)
	if err != nil {
		return config.Registry{}, nil, fmt.Errorf("parsing registry config: %w", err)
	}

	// Read user's configuration to populate registry tiles
	userConfigYAML, err := config.ReadConfig(ctx, docker)
	if err != nil {
		return config.Registry{}, nil, fmt.Errorf("reading user config: %w", err)
	}

	userConfig, err := config.ParseConfig(userConfigYAML)
	if err != nil {
		return config.Registry{}, nil, fmt.Errorf("parsing user config: %w", err)
	}

	// Populate registry tiles with user config
	for serverName, tile := range registry.Servers {
		if len(tile.Config) == 0 {
			if userServerConfig, hasUserConfig := userConfig[serverName]; hasUserConfig {
				tile.Config = userServerConfig
				registry.Servers[serverName] = tile
			}
		}
	}

	return registry, userConfig, nil
}
