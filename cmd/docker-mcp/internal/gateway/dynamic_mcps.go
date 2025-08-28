package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

// mcpFindTool implements a tool for finding MCP servers in the catalog
func (g *Gateway) createMcpFindTool(configuration Configuration) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-find",
		Description: "Find MCP servers in the current catalog by name or description. Returns matching servers with their details.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"query": {
					Type:        "string",
					Description: "Search query to find servers by name or description (case-insensitive)",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results to return (default: 10)",
				},
			},
			Required: []string{"query"},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
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

		if params.Query == "" {
			return nil, fmt.Errorf("query parameter is required")
		}

		if params.Limit <= 0 {
			params.Limit = 10
		}

		// Search through the catalog servers
		query := strings.ToLower(strings.TrimSpace(params.Query))
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
			
			// Check if it has tools that might match
			for _, tool := range server.Tools {
				toolNameLower := strings.ToLower(tool.Name)
				toolDescLower := strings.ToLower(tool.Description)
				
				if toolNameLower == query {
					match = true
					score = max(score, 90)
				} else if strings.Contains(toolNameLower, query) {
					match = true
					score = max(score, 40)
				} else if strings.Contains(toolDescLower, query) {
					match = true
					score = max(score, 30)
				}
			}
			
			// Check image name
			if server.Image != "" {
				imageLower := strings.ToLower(server.Image)
				if strings.Contains(imageLower, query) {
					match = true
					score = max(score, 20)
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
		for i := 0; i < len(matches)-1; i++ {
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
		var results []map[string]interface{}
		for _, match := range matches {
			serverInfo := map[string]interface{}{
				"name": match.Name,
			}

			if match.Server.Image != "" {
				serverInfo["image"] = match.Server.Image
			}

			if match.Server.Remote.URL != "" {
				serverInfo["remote_url"] = match.Server.Remote.URL
			}

			if match.Server.SSEEndpoint != "" {
				serverInfo["sse_endpoint"] = match.Server.SSEEndpoint
			}

			if len(match.Server.Tools) > 0 {
				var tools []map[string]string
				for _, tool := range match.Server.Tools {
					tools = append(tools, map[string]string{
						"name":        tool.Name,
						"description": tool.Description,
					})
				}
				serverInfo["tools"] = tools
			}

			if len(match.Server.Secrets) > 0 {
				var secrets []string
				for _, secret := range match.Server.Secrets {
					secrets = append(secrets, secret.Name)
				}
				serverInfo["required_secrets"] = secrets
			}

			serverInfo["long_lived"] = match.Server.LongLived
			serverInfo["disable_network"] = match.Server.DisableNetwork

			results = append(results, serverInfo)
		}

		response := map[string]interface{}{
			"query":         params.Query,
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

	return &ToolRegistration{
		Tool:    tool,
		Handler: handler,
	}
}

// ServerMatch represents a search result
type ServerMatch struct {
	Name   string
	Server catalog.Server
	Score  int
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}