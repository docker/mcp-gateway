package server

import (
	"context"

	"github.com/docker/mcp-cli/cmd/docker-mcp/internal/config"
)

func List(ctx context.Context, dockerClient config.VolumeInspecter) ([]string, error) {
	registryYAML, err := config.ReadRegistry(ctx, dockerClient)
	if err != nil {
		return nil, err
	}

	registry, err := config.ParseRegistryConfig(registryYAML)
	if err != nil {
		return nil, err
	}

	return registry.ServerNames(), nil
}
