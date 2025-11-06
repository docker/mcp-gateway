package gateway

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/gateway/embeddings"
	"github.com/docker/mcp-gateway/pkg/log"
)

// findServersByEmbedding finds relevant MCP servers using vector similarity search
func (g *Gateway) findServersByEmbedding(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	if g.embeddingsClient == nil {
		return nil, fmt.Errorf("embeddings client not initialized")
	}

	// Generate embedding for the query
	queryVector, err := generateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Search for similar servers in mcp-server-collection only
	results, err := g.embeddingsClient.SearchVectors(ctx, queryVector, &embeddings.SearchOptions{
		CollectionName: "mcp-server-collection",
		Limit:          limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	// Map results to servers from catalog
	var servers []map[string]any
	for _, result := range results {
		// Extract server name from metadata
		serverNameInterface, ok := result.Metadata["name"]
		if !ok {
			log.Logf("Warning: search result %d missing 'name' in metadata", result.ID)
			continue
		}

		serverName, ok := serverNameInterface.(string)
		if !ok {
			log.Logf("Warning: server name is not a string: %v", serverNameInterface)
			continue
		}

		// Look up the server in the catalog
		server, _, found := g.configuration.Find(serverName)
		if !found {
			log.Logf("Warning: server %s not found in catalog", serverName)
			continue
		}

		// Build server info map (same format as mcp-find)
		serverInfo := map[string]any{
			"name": serverName,
		}

		if server.Spec.Description != "" {
			serverInfo["description"] = server.Spec.Description
		}

		if len(server.Spec.Secrets) > 0 {
			var secrets []string
			for _, secret := range server.Spec.Secrets {
				secrets = append(secrets, secret.Name)
			}
			serverInfo["required_secrets"] = secrets
		}

		if len(server.Spec.Config) > 0 {
			serverInfo["config_schema"] = server.Spec.Config
		}

		serverInfo["long_lived"] = server.Spec.LongLived

		servers = append(servers, serverInfo)
	}

	return servers, nil
}
