package oci

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/docker"
)

func SelfContainedCatalog(ctx context.Context, dockerClient docker.Client, serverNames []string) (catalog.Catalog, []string, error) {
	result := catalog.Catalog{
		Servers: make(map[string]catalog.Server),
	}
	var resultServerNames []string

	for _, serverName := range serverNames {
		if ociRef, ok := strings.CutPrefix(serverName, "docker://"); ok {

			if err := dockerClient.PullImage(ctx, ociRef); err != nil {
				return catalog.Catalog{}, nil, fmt.Errorf("failed to pull OCI image %s: %w", ociRef, err)
			}

			inspect, err := dockerClient.InspectImage(ctx, ociRef)
			if err != nil {
				return catalog.Catalog{}, nil, fmt.Errorf("failed to inspect OCI image %s: %w", ociRef, err)
			}

			metadataLabel, exists := inspect.Config.Labels["io.docker.server.metadata"]
			if !exists {
				return catalog.Catalog{}, nil, fmt.Errorf("server name %s looks like an OCI ref but is missing the io.docker.server.metadata label", serverName)
			}

			var server catalog.Server
			if err := yaml.Unmarshal([]byte(metadataLabel), &server); err != nil {
				return catalog.Catalog{}, nil, fmt.Errorf("failed to parse metadata label for %s: %w", serverName, err)
			}

			server.Image = ociRef

			// Determine the server name to use
			finalServerName := serverName
			if server.Name != "" {
				finalServerName = server.Name
			}

			result.Servers[finalServerName] = server
			resultServerNames = append(resultServerNames, finalServerName)
		} else {
			// Non-OCI server names are passed through as-is
			resultServerNames = append(resultServerNames, serverName)
		}
	}

	return result, resultServerNames, nil
}
