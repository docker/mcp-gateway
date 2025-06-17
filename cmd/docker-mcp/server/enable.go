package server

import (
	"bytes"
	"context"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/catalog"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/config"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/docker"
)

func Disable(ctx context.Context, docker docker.Client, serverNames []string) error {
	return update(ctx, docker, nil, serverNames)
}

func Enable(ctx context.Context, docker docker.Client, serverNames []string) error {
	return update(ctx, docker, serverNames, nil)
}

func update(ctx context.Context, docker docker.Client, add []string, remove []string) error {
	// Read registry.yaml that contains which servers are enabled.
	registryYAML, err := config.ReadRegistry(ctx, docker)
	if err != nil {
		return fmt.Errorf("reading registry config: %w", err)
	}

	registry, err := config.ParseRegistryConfig(registryYAML)
	if err != nil {
		return fmt.Errorf("parsing registry config: %w", err)
	}

	catalog, err := catalog.Get(ctx)
	if err != nil {
		return err
	}

	updatedRegistry := config.Registry{
		Servers: map[string]config.Tile{},
	}

	// Keep only servers that are still in the catalog.
	for serverName := range registry.Servers {
		if serverSpec, found := catalog.Servers[serverName]; found {
			updatedRegistry.Servers[serverName] = config.Tile{
				Ref: serverSpec.Ref,
			}
		}
	}

	// Enable servers.
	for _, serverName := range add {
		if serverSpec, found := catalog.Servers[serverName]; found {
			updatedRegistry.Servers[serverName] = config.Tile{
				Ref: serverSpec.Ref,
			}
		} else {
			return fmt.Errorf("server %s not found in catalog", serverName)
		}
	}

	// Disable servers.
	for _, serverName := range remove {
		delete(updatedRegistry.Servers, serverName)
	}

	// Save it.
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(updatedRegistry); err != nil {
		return fmt.Errorf("encoding registry config: %w", err)
	}

	if err := config.WriteRegistry(buf.Bytes()); err != nil {
		return fmt.Errorf("writing registry config: %w", err)
	}

	return nil
}
