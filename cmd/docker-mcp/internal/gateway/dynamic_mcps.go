package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/oci"
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

	handler := func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

			if len(match.Server.Secrets) > 0 {
				var secrets []string
				for _, secret := range match.Server.Secrets {
					secrets = append(secrets, secret.Name)
				}
				serverInfo["required_secrets"] = secrets
			}

			serverInfo["long_lived"] = match.Server.LongLived

			results = append(results, serverInfo)
		}

		response := map[string]any{
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
func (g *Gateway) createMcpAddTool(configuration Configuration, clientConfig *clientConfig) *ToolRegistration {
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

		// Append the new server to the current serverNames
		configuration.serverNames = append(configuration.serverNames, serverName)

		// Update the current configuration state
		updatedServerNames := configuration.serverNames
		if err := g.reloadConfiguration(ctx, configuration, updatedServerNames, clientConfig); err != nil {
			return nil, fmt.Errorf("failed to reload configuration: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Successfully added server '%s'.", serverName),
			}},
		}, nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: handler,
	}
}

// mcpRemoveTool implements a tool for removing servers from the registry
func (g *Gateway) createMcpRemoveTool(_ Configuration, clientConfig *clientConfig) *ToolRegistration {
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

		// Remove the server from the current serverNames
		updatedServerNames := slices.DeleteFunc(slices.Clone(g.configuration.serverNames), func(name string) bool {
			return name == serverName
		})

		// Update the current configuration state
		g.configuration.serverNames = updatedServerNames

		if err := g.reloadConfiguration(ctx, g.configuration, updatedServerNames, clientConfig); err != nil {
			return nil, fmt.Errorf("failed to reload configuration: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Successfully removed server '%s'.", serverName),
			}},
		}, nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: handler,
	}
}

// mcpOfficialRegistryImportTool implements a tool for importing servers from official registry URLs
func (g *Gateway) createMcpOfficialRegistryImportTool(configuration Configuration, clientConfig *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-official-registry-import",
		Description: "Import MCP servers from an official registry URL. Fetches server definitions via HTTP GET and adds them to the local catalog.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"url": {
					Type:        "string",
					Description: "URL to fetch the official registry JSON from (must be a valid HTTP/HTTPS URL)",
				},
			},
			Required: []string{"url"},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			URL string `json:"url"`
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

		if params.URL == "" {
			return nil, fmt.Errorf("url parameter is required")
		}

		registryURL := strings.TrimSpace(params.URL)

		// Validate URL scheme
		if !strings.HasPrefix(registryURL, "http://") && !strings.HasPrefix(registryURL, "https://") {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error: URL must start with http:// or https://, got: %s", registryURL),
				}},
			}, nil
		}

		// Fetch servers from the URL
		servers, err := g.readServersFromURL(ctx, registryURL)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error fetching servers from URL %s: %v", registryURL, err),
				}},
			}, nil
		}

		if len(servers) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("No servers found at URL: %s", registryURL),
				}},
			}, nil
		}

		// Add the imported servers to the current configuration
		var importedServerNames []string
		for serverName, server := range servers {
			if _, exists := configuration.servers[serverName]; exists {
				log(fmt.Sprintf("Warning: server '%s' from URL %s overwrites existing server", serverName, registryURL))
			}
			configuration.servers[serverName] = server
			importedServerNames = append(importedServerNames, serverName)
		}

		// Update serverNames with imported servers
		for _, serverName := range importedServerNames {
			found := false
			for _, existing := range configuration.serverNames {
				if existing == serverName {
					found = true
					break
				}
			}
			if !found {
				configuration.serverNames = append(configuration.serverNames, serverName)
			}
		}

		// Reload configuration with updated server list
		if err := g.reloadConfiguration(ctx, configuration, configuration.serverNames, clientConfig); err != nil {
			return nil, fmt.Errorf("failed to reload configuration: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Successfully imported %d servers from %s: %s", len(importedServerNames), registryURL, strings.Join(importedServerNames, ", ")),
			}},
		}, nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: handler,
	}
}

// readServersFromURL fetches and parses server definitions from a URL
func (g *Gateway) readServersFromURL(ctx context.Context, url string) (map[string]catalog.Server, error) {
	servers := make(map[string]catalog.Server)

	log(fmt.Sprintf("  - Reading servers from URL: %s", url))

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set a reasonable user agent
	req.Header.Set("User-Agent", "docker-mcp-gateway/1.0.0")

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Try to parse as oci.ServerDetail (the new structure)
	var serverDetail oci.ServerDetail
	if err := json.Unmarshal(body, &serverDetail); err == nil && serverDetail.Name != "" {
		// Successfully parsed as ServerDetail - convert to catalog.Server
		server := serverDetail.ToCatalogServer()

		serverName := serverDetail.Name
		servers[serverName] = server
		log(fmt.Sprintf("  - Added server '%s' from URL %s", serverName, url))
		return servers, nil
	}

	return nil, fmt.Errorf("unable to parse response as OCI catalog or direct catalog format")
}

//nolint:gofmt
type configValue struct {
	Server string      `json:"server"`
	Key    string      `json:"key"`
	Value  interface{} `json:"value"`
}

// mcpConfigSetTool implements a tool for setting configuration values for MCP servers
func (g *Gateway) createMcpConfigSetTool(configuration Configuration, clientConfig *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-config-set",
		Description: "Set configuration values for MCP servers. Creates or updates server configuration with the specified key-value pairs.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"server": {
					Type:        "string",
					Description: "Name of the MCP server to configure",
				},
				"key": {
					Type:        "string",
					Description: "Configuration key to set",
				},
				"value": {
					Description: "Configuration value to set (can be string, number, boolean, or object)",
				},
			},
			Required: []string{"server", "key", "value"},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params configValue

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

		if params.Server == "" {
			return nil, fmt.Errorf("server parameter is required")
		}

		if params.Key == "" {
			return nil, fmt.Errorf("key parameter is required")
		}

		serverName := strings.TrimSpace(params.Server)
		configKey := strings.TrimSpace(params.Key)

		// Check if server exists in catalog (optional check - we can configure servers that don't exist yet)
		_, _, serverExists := configuration.Find(serverName)

		// Initialize the server's config map if it doesn't exist
		if configuration.config[serverName] == nil {
			configuration.config[serverName] = make(map[string]any)
		}

		// Set the configuration value
		oldValue := configuration.config[serverName][configKey]
		configuration.config[serverName][configKey] = params.Value

		// Log the configuration change
		log(fmt.Sprintf("  - Set config for server '%s': %s = %v", serverName, configKey, params.Value))

		// Reload configuration with current server list to apply changes
		if err := g.reloadConfiguration(ctx, configuration, configuration.serverNames, clientConfig); err != nil {
			return nil, fmt.Errorf("failed to reload configuration: %w", err)
		}

		var resultMessage string
		if oldValue != nil {
			resultMessage = fmt.Sprintf("Successfully updated config for server '%s': %s = %v (was: %v)", serverName, configKey, params.Value, oldValue)
		} else {
			resultMessage = fmt.Sprintf("Successfully set config for server '%s': %s = %v", serverName, configKey, params.Value)
		}

		if !serverExists {
			resultMessage += fmt.Sprintf(" (Note: server '%s' is not in the current catalog)", serverName)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: resultMessage,
			}},
		}, nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: handler,
	}
}

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
