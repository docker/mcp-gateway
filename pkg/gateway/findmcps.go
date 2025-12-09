package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/gateway/embeddings"
	"github.com/docker/mcp-gateway/pkg/log"
)

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ServerMatch represents a search result
type ServerMatch struct {
	Name   string
	Server catalog.Server
	Score  int
}

func keywordStrategy(configuration Configuration) mcp.ToolHandler {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Prompt string `json:"prompt"`
			Limit  int    `json:"limit"`
		}

		if req.Params.Arguments == nil {
			return nil, fmt.Errorf("missing arguments")
		}

		paramsBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}

		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to parse arguments: %w", err)
		}

		if params.Prompt == "" {
			return nil, fmt.Errorf("query parameter is required")
		}

		if params.Limit <= 0 {
			params.Limit = 10
		}

		// Search through the catalog servers
		query := strings.ToLower(strings.TrimSpace(params.Prompt))
		var matches []ServerMatch

		for serverName, server := range configuration.servers {
			match := false
			score := 0

			// Check server name (exact match gets higher score)
			serverNameLower := strings.ToLower(serverName)
			if serverNameLower == query {
				match = true
				score = 100
			} else if strings.Contains(serverNameLower, query) {
				match = true
				score = 50
			}

			// Check server title
			if server.Title != "" {
				titleLower := strings.ToLower(server.Title)
				if titleLower == query {
					match = true
					score = maxInt(score, 97)
				} else if strings.Contains(titleLower, query) {
					match = true
					score = maxInt(score, 47)
				}
			}

			// Check server description
			if server.Description != "" {
				descriptionLower := strings.ToLower(server.Description)
				if descriptionLower == query {
					match = true
					score = maxInt(score, 95)
				} else if strings.Contains(descriptionLower, query) {
					match = true
					score = maxInt(score, 45)
				}
			}

			// Check if it has tools that might match
			for _, tool := range server.Tools {
				toolNameLower := strings.ToLower(tool.Name)
				toolDescLower := strings.ToLower(tool.Description)

				if toolNameLower == query {
					match = true
					score = maxInt(score, 90)
				} else if strings.Contains(toolNameLower, query) {
					match = true
					score = maxInt(score, 40)
				} else if strings.Contains(toolDescLower, query) {
					match = true
					score = maxInt(score, 30)
				}
			}

			// Check image name
			if server.Image != "" {
				imageLower := strings.ToLower(server.Image)
				if strings.Contains(imageLower, query) {
					match = true
					score = maxInt(score, 20)
				}
			}

			if match {
				matches = append(matches, ServerMatch{
					Name:   serverName,
					Server: server,
					Score:  score,
				})
			}
		}

		// Sort matches by score (higher scores first)
		for i := range len(matches) - 1 {
			for j := i + 1; j < len(matches); j++ {
				if matches[i].Score < matches[j].Score {
					matches[i], matches[j] = matches[j], matches[i]
				}
			}
		}

		// Limit results
		if len(matches) > params.Limit {
			matches = matches[:params.Limit]
		}

		// Format results
		var results []map[string]any
		for _, match := range matches {
			serverInfo := map[string]any{
				"name": match.Name,
			}

			if match.Server.Description != "" {
				serverInfo["description"] = match.Server.Description
			}

			if len(match.Server.Secrets) > 0 {
				var secrets []string
				for _, secret := range match.Server.Secrets {
					secrets = append(secrets, secret.Name)
				}
				serverInfo["required_secrets"] = secrets
			}

			if len(match.Server.Config) > 0 {
				serverInfo["config_schema"] = match.Server.Config
			}

			serverInfo["long_lived"] = match.Server.LongLived

			results = append(results, serverInfo)
		}

		response := map[string]any{
			"prompt":        params.Prompt,
			"total_matches": len(results),
			"servers":       results,
		}

		responseBytes, err := json.Marshal(response)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(responseBytes)}},
		}, nil
	}
}

func embeddingStrategy(g *Gateway) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Prompt string `json:"prompt"`
			Limit  int    `json:"limit"`
		}

		if req.Params.Arguments == nil {
			return nil, fmt.Errorf("missing arguments")
		}

		paramsBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}

		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to parse arguments: %w", err)
		}

		if params.Prompt == "" {
			return nil, fmt.Errorf("query parameter is required")
		}

		if params.Limit <= 0 {
			params.Limit = 10
		}

		// Use vector similarity search to find relevant servers
		results, err := g.findServersByEmbedding(ctx, params.Prompt, params.Limit)
		if err != nil {
			return nil, fmt.Errorf("failed to find servers: %w", err)
		}

		response := map[string]any{
			"prompt":        params.Prompt,
			"total_matches": len(results),
			"servers":       results,
		}

		responseBytes, err := json.Marshal(response)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(responseBytes)}},
		}, nil
	}
}

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
