package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/config"
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

// mcpAddTool implements a tool for adding new servers to the registry
func (g *Gateway) createMcpAddTool(configuration Configuration) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-add",
		Description: "Add a new MCP server to the registry and reload the configuration. The server must exist in the catalog.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the MCP server to add to the registry (must exist in catalog)",
				},
			},
			Required: []string{"name"},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Name string `json:"name"`
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

		if params.Name == "" {
			return nil, fmt.Errorf("name parameter is required")
		}

		serverName := strings.TrimSpace(params.Name)

		// Check if server exists in catalog
		_, _, found := configuration.Find(serverName)
		if !found {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error: Server '%s' not found in catalog. Use mcp-find to search for available servers.", serverName),
				}},
			}, nil
		}

		// Read current registry
		registry, err := g.readRegistryConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read registry: %w", err)
		}

		// Check if server is already in registry
		if _, exists := registry.Servers[serverName]; exists {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Server '%s' is already enabled in the registry.", serverName),
				}},
			}, nil
		}

		// Add server to registry  
		if registry.Servers == nil {
			registry.Servers = make(map[string]RegistryTile)
		}
		registry.Servers[serverName] = RegistryTile{
			Ref: serverName, // Use the server name as ref for catalog servers
		}

		// Write updated registry
		if err := g.writeRegistryConfig(registry); err != nil {
			return nil, fmt.Errorf("failed to write registry: %w", err)
		}

		// Trigger configuration reload
		newConfiguration, err := g.readConfigurationForReload(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read updated configuration: %w", err)
		}

		if err := g.reloadConfiguration(ctx, newConfiguration, nil, nil); err != nil {
			return nil, fmt.Errorf("failed to reload configuration: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Successfully added server '%s' to the registry and reloaded configuration.", serverName),
			}},
		}, nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: handler,
	}
}

// mcpRemoveTool implements a tool for removing servers from the registry
func (g *Gateway) createMcpRemoveTool(configuration Configuration) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-remove",
		Description: "Remove an MCP server from the registry and reload the configuration. This will disable the server.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the MCP server to remove from the registry",
				},
			},
			Required: []string{"name"},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Name string `json:"name"`
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

		if params.Name == "" {
			return nil, fmt.Errorf("name parameter is required")
		}

		serverName := strings.TrimSpace(params.Name)

		// Read current registry
		registry, err := g.readRegistryConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read registry: %w", err)
		}

		// Check if server is in registry
		if _, exists := registry.Servers[serverName]; !exists {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Server '%s' is not found in the registry (not currently enabled).", serverName),
				}},
			}, nil
		}

		// Remove server from registry
		delete(registry.Servers, serverName)

		// Write updated registry
		if err := g.writeRegistryConfig(registry); err != nil {
			return nil, fmt.Errorf("failed to write registry: %w", err)
		}

		// Trigger configuration reload
		newConfiguration, err := g.readConfigurationForReload(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read updated configuration: %w", err)
		}

		if err := g.reloadConfiguration(ctx, newConfiguration, nil, nil); err != nil {
			return nil, fmt.Errorf("failed to reload configuration: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Successfully removed server '%s' from the registry and reloaded configuration.", serverName),
			}},
		}, nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: handler,
	}
}

// Helper methods for registry management
func (g *Gateway) readRegistryConfig(ctx context.Context) (*RegistryConfig, error) {
	registry, err := g.configurator.(*FileBasedConfiguration).readRegistry(ctx)
	if err != nil {
		return nil, err
	}
	
	// Convert from config.Registry to our local RegistryConfig type
	servers := make(map[string]RegistryTile)
	for name, tile := range registry.Servers {
		servers[name] = RegistryTile{
			Ref:    tile.Ref,
			Config: tile.Config,
		}
	}
	
	return &RegistryConfig{
		Servers: servers,
	}, nil
}

func (g *Gateway) writeRegistryConfig(registry *RegistryConfig) error {
	// Convert back to config.Registry format
	configRegistry := config.Registry{
		Servers: make(map[string]config.Tile),
	}
	
	for name, tile := range registry.Servers {
		configRegistry.Servers[name] = config.Tile{
			Ref:    tile.Ref,
			Config: tile.Config,
		}
	}
	
	// Marshal to YAML
	output := struct {
		Registry map[string]config.Tile `yaml:"registry"`
	}{
		Registry: configRegistry.Servers,
	}
	
	data, err := yaml.Marshal(output)
	if err != nil {
		return err
	}
	
	return g.writeRegistryFile(data)
}

func (g *Gateway) writeRegistryFile(data []byte) error {
	// Use config.WriteRegistry if available, or manually write
	return config.WriteRegistry(data)
}

func (g *Gateway) readConfigurationForReload(ctx context.Context) (Configuration, error) {
	configuration, _, _, err := g.configurator.Read(ctx)
	return configuration, err
}

// Helper types
type RegistryConfig struct {
	Servers map[string]RegistryTile
}

type RegistryTile struct {
	Ref    string         `yaml:"ref"`
	Config map[string]any `yaml:"config,omitempty"`
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}