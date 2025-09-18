package server

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/docker"
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
