package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oci"
)

// readServersFromURL fetches and parses server definitions from a URL
//
//nolint:unused // TODO: This function will be used when registry import feature is enabled
func (g *Gateway) readServersFromURL(ctx context.Context, url string) (map[string]catalog.Server, error) {
	servers := make(map[string]catalog.Server)

	log.Log(fmt.Sprintf("  - Reading servers from URL: %s", url))

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
		log.Log(fmt.Sprintf("  - Added server '%s' from URL %s", serverName, url))
		return servers, nil
	}

	return nil, fmt.Errorf("unable to parse response as OCI catalog or direct catalog format")
}

//nolint:unused // TODO: This handler will be used when registry import feature is enabled
func registryImportHandler(g *Gateway, configuration Configuration) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		// Add the imported servers to the current configuration and build detailed summary
		var importedServerNames []string
		var serverSummaries []string

		for serverName, server := range servers {
			if _, exists := configuration.servers[serverName]; exists {
				log.Log(fmt.Sprintf("Warning: server '%s' from URL %s overwrites existing server", serverName, registryURL))
			}
			configuration.servers[serverName] = server
			importedServerNames = append(importedServerNames, serverName)

			// Build detailed summary for this server
			summary := fmt.Sprintf("• %s", serverName)

			if server.Description != "" {
				summary += fmt.Sprintf("\n  Description: %s", server.Description)
			}

			if server.Image != "" {
				summary += fmt.Sprintf("\n  Image: %s", server.Image)
			}

			// List required secrets
			if len(server.Secrets) > 0 {
				var secretNames []string
				for _, secret := range server.Secrets {
					secretNames = append(secretNames, secret.Name)
				}
				summary += fmt.Sprintf("\n  Required Secrets: %s", strings.Join(secretNames, ", "))
				summary += "\n  ⚠️  Configure these secrets before using this server"
			}

			// List configuration schemas available
			if len(server.Config) > 0 {
				summary += fmt.Sprintf("\n  Configuration Schemas: %d available", len(server.Config))
				summary += "\n  ℹ️  Use mcp-config-set to configure optional settings"
			}

			if server.LongLived {
				summary += "\n  🔄 Long-lived server (stays running)"
			}

			serverSummaries = append(serverSummaries, summary)
		}

		// Create comprehensive result message
		resultText := fmt.Sprintf("Successfully imported %d servers from %s\n\n", len(importedServerNames), registryURL)
		resultText += strings.Join(serverSummaries, "\n\n")

		if len(importedServerNames) > 0 {
			resultText += fmt.Sprintf("\n\n✅ Servers ready to use: %s", strings.Join(importedServerNames, ", "))
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: resultText,
			}},
		}, nil
	}
}
