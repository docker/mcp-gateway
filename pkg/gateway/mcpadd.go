package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/contextkeys"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oauth"
	"github.com/docker/mcp-gateway/pkg/oci"
)

// mcpAddTool implements a tool for adding new servers to the registry
func (g *Gateway) createMcpAddTool(clientConfig *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-add",
		Description: "Add a new MCP server to the session. The server must exist in the catalog.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the MCP server to add to the registry (must exist in catalog)",
				},
				"activate": {
					Type:        "boolean",
					Description: "Activate all of the server's tools in the current session",
				},
			},
			Required: []string{"name"},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Name     string `json:"name"`
			Activate bool   `json:"activate"`
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
		serverConfig, _, found := g.configuration.Find(serverName)
		if !found {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error: Server '%s' not found in catalog. Use mcp-find to search for available servers.", serverName),
				}},
			}, nil
		}

		// Append the new server to the current serverNames if not already present
		found = slices.Contains(g.configuration.serverNames, serverName)
		if !found {
			g.configuration.serverNames = append(g.configuration.serverNames, serverName)
		}

		// Secrets validation removed - se:// URIs are resolved at runtime by Docker Desktop
		// Containers will fail to start if secrets are missing in the secrets engine
		var missingSecrets []string

		// Check if all required config values are set and validate against schema
		var missingConfig []string
		if serverConfig != nil && len(serverConfig.Spec.Config) > 0 {
			canonicalServerName := oci.CanonicalizeServerName(serverName)
			serverConfigMap := g.configuration.config[canonicalServerName]

			for _, configItem := range serverConfig.Spec.Config {
				// Config items should be schema objects with a "name" property
				schemaMap, ok := configItem.(map[string]any)
				if !ok {
					continue
				}

				// Get the name field - this identifies which config to validate
				configName, ok := schemaMap["name"].(string)
				if !ok || configName == "" {
					continue
				}

				// Get the actual config value to validate
				if serverConfigMap == nil {
					missingConfig = append(missingConfig, fmt.Sprintf("%s (missing)", configName))
					continue
				}

				configValue := serverConfigMap

				// Convert the schema map to a jsonschema.Schema for validation
				schemaBytes, err := json.Marshal(schemaMap)
				if err != nil {
					missingConfig = append(missingConfig, fmt.Sprintf("%s (invalid schema)", configName))
					continue
				}

				var schema jsonschema.Schema
				if err := json.Unmarshal(schemaBytes, &schema); err != nil {
					missingConfig = append(missingConfig, fmt.Sprintf("%s (invalid schema)", configName))
					continue
				}

				// Resolve the schema
				resolved, err := schema.Resolve(nil)
				if err != nil {
					missingConfig = append(missingConfig, fmt.Sprintf("%s (schema resolution failed)", configName))
					continue
				}

				// Validate the config value against the schema
				if err := resolved.Validate(configValue); err != nil {
					// Extract a helpful error message
					errMsg := err.Error()
					if len(errMsg) > 100 {
						errMsg = errMsg[:97] + "..."
					}
					missingConfig = append(missingConfig, fmt.Sprintf("%s (%s)", configName, errMsg))
				}
			}
		}

		// If secrets or config are missing, handle based on client type
		if len(missingSecrets) > 0 || len(missingConfig) > 0 {
			// Check if the client is nanobot
			clientName := ""
			if req.Session.InitializeParams().ClientInfo != nil {
				clientName = req.Session.InitializeParams().ClientInfo.Name
			}

			if clientName == "nanobot" && len(missingSecrets) > 0 {
				// For nanobot, return the interactive UI (only for secrets)
				return secretInput(missingSecrets, serverName), nil
			}

			// For other clients, return an error with command line instructions
			var instructions []string
			var missingItems []string

			if len(missingSecrets) > 0 {
				missingItems = append(missingItems, fmt.Sprintf("secrets (%s)", strings.Join(missingSecrets, ", ")))
				instructions = append(instructions, "\nRequired secrets:")
				for _, secret := range missingSecrets {
					instructions = append(instructions, fmt.Sprintf("  docker mcp secret set %s=<value>", secret))
				}
			}

			if len(missingConfig) > 0 {
				missingItems = append(missingItems, fmt.Sprintf("config (%s)", strings.Join(missingConfig, ", ")))
				instructions = append(instructions, fmt.Sprintf("\nRequired configuration: %s", strings.Join(missingConfig, ", ")))
				instructions = append(instructions, "Use the mcp-config-set tool to configure these values.")
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error: Cannot add server '%s'. Missing required %s.\n\nThe server was not added. Please configure these first:%s",
						serverName, strings.Join(missingItems, " and "), strings.Join(instructions, "\n")),
				}},
			}, nil
		}

		// Pull the Docker image before trying to use the server
		if serverConfig.Spec.Image != "" {
			log.Log(fmt.Sprintf("Pulling image for server '%s': %s", serverName, serverConfig.Spec.Image))
			if err := g.docker.PullImage(ctx, serverConfig.Spec.Image); err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: fmt.Sprintf("Error: Failed to pull image '%s' for server '%s'.\n\nDetails: %v\n\nThe server was not added. Please check the image name and your network connection.",
							serverConfig.Spec.Image, serverName, err),
					}},
				}, nil
			}
		}

		oldCaps, err := g.reloadServerCapabilities(ctx, serverName, clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to reload configuration: %w", err)
		}

		// Get client name to determine whether to activate tools
		clientName := ""
		if req.Session.InitializeParams().ClientInfo != nil {
			clientName = req.Session.InitializeParams().ClientInfo.Name
		}
		clientNameLower := strings.ToLower(clientName)

		// Only activate tools if activate is true AND client name doesn't contain "claude"
		// (Claude clients auto-refresh their tool list, so they don't need explicit activation)
		shouldActivate := params.Activate && !strings.Contains(clientNameLower, "claude")

		if shouldActivate {
			// Now update g.mcpServer with the new capabilities
			g.capabilitiesMu.Lock()
			newCaps := g.allCapabilities(serverName)
			if err := g.updateServerCapabilities(serverName, oldCaps, newCaps, nil); err != nil {
				g.capabilitiesMu.Unlock()
				return nil, fmt.Errorf("failed to update server capabilities: %w", err)
			}
			g.capabilitiesMu.Unlock()
		}

		// Persist configuration if session name is set
		if err := g.configuration.Persist(); err != nil {
			log.Log("Warning: Failed to persist configuration:", err)
		}

		// Get the list of tools that were just added from this server
		var addedTools []*mcp.Tool
		g.capabilitiesMu.RLock()
		if availableCaps := g.serverAvailableCapabilities[serverName]; availableCaps != nil {
			for _, toolReg := range availableCaps.Tools {
				addedTools = append(addedTools, toolReg.Tool)
			}
		}
		g.capabilitiesMu.RUnlock()

		// Build the response text
		responseText := fmt.Sprintf("Successfully added %d tools in server '%s'. Assume that it is fully configured and ready to use.", len(addedTools), serverName)

		// Include the JSON representation of the newly added tools if client name contains "cagent" or "claude"
		shouldSendTools := len(addedTools) > 0 && strings.Contains(clientNameLower, "claude")

		if shouldSendTools {
			// Create a tools list response matching the format from tools/list
			toolsList := make([]map[string]any, 0, len(addedTools))
			for _, tool := range addedTools {
				toolMap := map[string]any{
					"name":        tool.Name,
					"description": tool.Description,
				}
				if tool.InputSchema != nil {
					toolMap["inputSchema"] = tool.InputSchema
				}
				toolsList = append(toolsList, toolMap)
			}

			// Convert to JSON
			toolsJSON, err := json.MarshalIndent(map[string]any{
				"tools": toolsList,
			}, "", "  ")
			if err == nil {
				responseText += "\n\nNewly added tools:\n```json\n" + string(toolsJSON) + "\n```"
			}
		}

		// Register DCR client and make sure OAuth provider is started if this is a remote OAuth server
		if g.McpOAuthDcrEnabled && serverConfig != nil && serverConfig.Spec.IsRemoteOAuthServer() {
			authorized, oauthText := g.getRemoteOAuthServerStatus(ctx, serverName, req, shouldSendTools)
			if !authorized {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: oauthText,
					}},
				}, nil
			}
			responseText = oauthText
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: responseText,
			}},
		}, nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-add", handler),
	}
}

// shortenURL creates a shortened URL using Bitly's API
// It returns the shortened URL or an error if the request fails
func shortenURL(ctx context.Context, longURL string) (string, error) {
	// Get Bitly API token from environment or secrets
	apiToken := os.Getenv("BITLY_ACCESS_TOKEN")
	if apiToken == "" {
		return "", fmt.Errorf("BITLY_ACCESS_TOKEN not set")
	}

	// Create the request payload
	payload := map[string]string{
		"long_url": longURL,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create HTTP request to Bitly API
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api-ssl.bitly.com/v4/shorten", strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	// Make the request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to shorten URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("bitly API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var response struct {
		Link string `json:"link"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Link == "" {
		return "", fmt.Errorf("empty link in response")
	}

	return response.Link, nil
}

// addRemoteOAuthServer handles the OAuth setup for a remote OAuth server
// It registers the provider, starts it, and handles authorization through elicitation or direct URL
// Returns the text message for the CallToolResult
func (g *Gateway) getRemoteOAuthServerStatus(ctx context.Context, serverName string, req *mcp.CallToolRequest, shouldSendTools bool) (bool, string) {
	// Check if provider already exists
	g.providersMu.RLock()
	_, providerExists := g.oauthProviders[serverName]
	g.providersMu.RUnlock()

	// Only register and start provider if it doesn't already exist
	if !providerExists {
		// Register DCR client with DD so user can authorize
		if err := oauth.RegisterProviderForLazySetup(ctx, serverName); err != nil {
			log.Logf("Warning: Failed to register OAuth provider for %s: %v", serverName, err)
		}

		// Start provider
		g.startProvider(ctx, serverName)
	}

	// Check if current serverSession supports elicitations
	if req.Session.InitializeParams().Capabilities != nil && req.Session.InitializeParams().Capabilities.Elicitation != nil {
		// Elicit a response from the client asking whether to open a browser for authorization
		elicitResult, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
			Message: fmt.Sprintf("Would you like to open a browser to authorize the '%s' server?", serverName),
			RequestedSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"authorize": {
						Type:        "boolean",
						Description: "Whether to open the browser for authorization",
					},
				},
				Required: []string{"authorize"},
			},
		})
		if err != nil {
			log.Logf("Warning: Failed to elicit authorization response for %s: %v", serverName, err)
			return false, "Client rejected eliciation to authorize"
		} else if elicitResult.Action == "accept" && elicitResult.Content != nil {
			// Check if user authorized
			if authorize, ok := elicitResult.Content["authorize"].(bool); ok && authorize {
				// User agreed to authorize, call the OAuth authorize function
				client := desktop.NewAuthClient()
				authResponse, err := client.PostOAuthApp(ctx, serverName, "", false)
				if err != nil {
					log.Logf("Warning: Failed to start OAuth flow for %s: %v", serverName, err)
					return false, "unable to trigger OAuth Flow"
				} else if authResponse.BrowserURL != "" {
					log.Logf("Opening browser for authentication: %s", authResponse.BrowserURL)
				} else {
					log.Logf("Warning: OAuth provider for %s does not exist", serverName)
					return false, "unable to trigger OAuth Flow"
				}
			}
		}

		return true, fmt.Sprintf("Successfully added server '%s'. Authorization completed.", serverName)
	}

	// Check if user is already authorized by checking the credential helper (only if provider exists)
	if providerExists {
		// Create a credential helper to check token status
		credHelper := oauth.NewOAuthCredentialHelper()
		tokenStatus, err := credHelper.GetTokenStatus(ctx, serverName)
		if err == nil && tokenStatus.Valid {
			// User is already authorized, skip the OAuth URL generation
			if shouldSendTools {
				return true, fmt.Sprintf("You will need to authorize this server with: docker mcp oauth authorize %s.\n  After authorizing, reconnect your agent to the MCP gateway.", serverName)
			}
			return true, fmt.Sprintf("Successfully added server '%s'. Server is authorized.", serverName)
		}
	}

	// Client doesn't support elicitations, get the login link and include it in the response
	client := desktop.NewAuthClient()
	// Set context flag to enable disableAutoOpen parameter
	ctxWithFlag := context.WithValue(ctx, contextkeys.OAuthInterceptorEnabledKey, true)
	// disable auto-open
	authResponse, err := client.PostOAuthApp(ctxWithFlag, serverName, "", true)
	if err != nil {
		log.Logf("Warning: Failed to get OAuth URL for %s: %v", serverName, err)
		return false, "Unable to get OAuth URL"
	} else if authResponse.BrowserURL != "" {
		// Try to shorten the URL using Bitly
		shortURL, err := shortenURL(ctx, authResponse.BrowserURL)
		var displayLink string
		if err != nil {
			// If shortening fails, use the original URL
			log.Logf("Warning: Failed to shorten URL for %s: %v", serverName, err)
			displayLink = fmt.Sprintf("[Click here to authorize](%s)", authResponse.BrowserURL)
		} else {
			// Use the shortened URL in the markdown link
			displayLink = fmt.Sprintf("[Click here to authorize](%s)", shortURL)
		}

		return false, fmt.Sprintf("Successfully added server '%s'. To authorize this server, please %s", serverName, displayLink)
	}

	return false, fmt.Sprintf("Successfully added server '%s'. You will need to authorize this server with: docker mcp oauth authorize %s", serverName, serverName)
}
