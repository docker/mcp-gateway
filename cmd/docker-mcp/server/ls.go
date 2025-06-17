package server

import (
	"context"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/config"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/docker"
)

func List(ctx context.Context, docker docker.Client) ([]string, error) {
	registryYAML, err := config.ReadRegistry(ctx, docker)
	if err != nil {
		return nil, err
	}

	registry, err := config.ParseRegistryConfig(registryYAML)
	if err != nil {
		return nil, err
	}

	return registry.ServerNames(), nil
}
