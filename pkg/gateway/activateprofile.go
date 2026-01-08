package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oci"
)

// ActivateProfileResult contains the result of profile activation
type ActivateProfileResult struct {
	ActivatedServers []string
	SkippedServers   []string
	ErrorMessage     string
}

// ActivateProfile activates a profile by name, loading its servers into the gateway
func (g *Gateway) ActivateProfile(ctx context.Context, profileName string) error {
	// Create database connection
	dao, err := db.New()
	if err != nil {
		return fmt.Errorf("failed to create database client: %w", err)
	}
	defer dao.Close()

	// Create a temporary WorkingSetConfiguration to load the profile
	wsConfig := NewWorkingSetConfiguration(
		Config{WorkingSet: profileName},
		oci.NewService(),
		g.docker,
	)

	// Load the full profile configuration using the existing readOnce method
	profileConfig, err := wsConfig.readOnce(ctx, dao)
	if err != nil {
		return fmt.Errorf("failed to load profile '%s': %w", profileName, err)
	}

	// Filter servers: only activate servers that are not already active
	var serversToActivate []string
	var skippedServers []string

	for _, serverName := range profileConfig.serverNames {
		if slices.Contains(g.configuration.serverNames, serverName) {
			skippedServers = append(skippedServers, serverName)
		} else {
			serversToActivate = append(serversToActivate, serverName)
		}
	}

	// If no servers to activate, return early
	if len(serversToActivate) == 0 {
		if len(skippedServers) > 0 {
			log.Log(fmt.Sprintf("- All servers from profile '%s' are already active: %s", profileName, strings.Join(skippedServers, ", ")))
		} else {
			log.Log(fmt.Sprintf("- No new servers to activate from profile '%s'", profileName))
		}
		return nil
	}

	// Validate ALL servers before activating any (all-or-nothing)
	var validationErrors []serverValidation

	for _, serverName := range serversToActivate {
		serverConfig := profileConfig.servers[serverName]
		validation := serverValidation{serverName: serverName}

		// Check if all required secrets are set
		for _, secret := range serverConfig.Secrets {
			if value, exists := profileConfig.secrets[secret.Name]; !exists || value == "" {
				validation.missingSecrets = append(validation.missingSecrets, secret.Name)
			}
		}

		// Check if all required config values are set and validate against schema
		if len(serverConfig.Config) > 0 {
			// Get config from profile
			serverConfigMap := profileConfig.config[serverName]

			for _, configItem := range serverConfig.Config {
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
					validation.missingConfig = append(validation.missingConfig, fmt.Sprintf("%s (missing)", configName))
					continue
				}

				configValue := serverConfigMap

				// Convert the schema map to a jsonschema.Schema for validation
				schemaBytes, err := json.Marshal(schemaMap)
				if err != nil {
					validation.missingConfig = append(validation.missingConfig, fmt.Sprintf("%s (invalid schema)", configName))
					continue
				}

				var schema jsonschema.Schema
				if err := json.Unmarshal(schemaBytes, &schema); err != nil {
					validation.missingConfig = append(validation.missingConfig, fmt.Sprintf("%s (invalid schema)", configName))
					continue
				}

				// Resolve the schema
				resolved, err := schema.Resolve(nil)
				if err != nil {
					validation.missingConfig = append(validation.missingConfig, fmt.Sprintf("%s (schema resolution failed)", configName))
					continue
				}

				// Validate the config value against the schema
				if err := resolved.Validate(configValue); err != nil {
					// Extract a helpful error message
					errMsg := err.Error()
					if len(errMsg) > 100 {
						errMsg = errMsg[:97] + "..."
					}
					validation.missingConfig = append(validation.missingConfig, fmt.Sprintf("%s (%s)", configName, errMsg))
				}
			}
		}

		// Validate that Docker image can be pulled
		if serverConfig.Image != "" {
			log.Log(fmt.Sprintf("Validating image for server '%s': %s", serverName, serverConfig.Image))
			if err := g.docker.PullImage(ctx, serverConfig.Image); err != nil {
				validation.imagePullError = err
			}
		}

		// Collect validation errors
		if len(validation.missingSecrets) > 0 || len(validation.missingConfig) > 0 || validation.imagePullError != nil {
			validationErrors = append(validationErrors, validation)
		}
	}

	// If any validation errors, return detailed error message
	if len(validationErrors) > 0 {
		var errorMessages []string
		errorMessages = append(errorMessages, fmt.Sprintf("Cannot activate profile '%s'. Validation failed for %d server(s):", profileName, len(validationErrors)))

		for _, validation := range validationErrors {
			errorMessages = append(errorMessages, fmt.Sprintf("\nServer '%s':", validation.serverName))

			if len(validation.missingSecrets) > 0 {
				errorMessages = append(errorMessages, fmt.Sprintf("  Missing secrets: %s", strings.Join(validation.missingSecrets, ", ")))
			}

			if len(validation.missingConfig) > 0 {
				errorMessages = append(errorMessages, fmt.Sprintf("  Missing/invalid config: %s", strings.Join(validation.missingConfig, ", ")))
			}

			if validation.imagePullError != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("  Image pull failed: %v", validation.imagePullError))
			}
		}

		return fmt.Errorf("%s", strings.Join(errorMessages, "\n"))
	}

	// All validations passed - merge configuration into current gateway
	var activatedServers []string

	// Merge secrets once (they're already namespaced in profileConfig)
	for secretName, secretValue := range profileConfig.secrets {
		g.configuration.secrets[secretName] = secretValue
	}

	for _, serverName := range serversToActivate {
		// Add server name to the list
		g.configuration.serverNames = append(g.configuration.serverNames, serverName)

		// Add server definition
		g.configuration.servers[serverName] = profileConfig.servers[serverName]

		// Merge server config
		if profileConfig.config[serverName] != nil {
			if g.configuration.config == nil {
				g.configuration.config = make(map[string]map[string]any)
			}
			g.configuration.config[serverName] = profileConfig.config[serverName]
		}

		// Merge tools configuration
		if tools, exists := profileConfig.tools.ServerTools[serverName]; exists {
			if g.configuration.tools.ServerTools == nil {
				g.configuration.tools.ServerTools = make(map[string][]string)
			}
			g.configuration.tools.ServerTools[serverName] = tools
		}

		// Reload server capabilities
		oldCaps, err := g.reloadServerCapabilities(ctx, serverName, nil)
		if err != nil {
			log.Log(fmt.Sprintf("Warning: Failed to reload capabilities for server '%s': %v", serverName, err))
			// Continue with other servers even if this one fails
			continue
		}

		// Update g.mcpServer with the new capabilities
		g.capabilitiesMu.Lock()
		newCaps := g.allCapabilities(serverName)
		if err := g.updateServerCapabilities(serverName, oldCaps, newCaps, nil); err != nil {
			g.capabilitiesMu.Unlock()
			log.Log(fmt.Sprintf("Warning: Failed to update server capabilities for '%s': %v", serverName, err))
			// Continue with other servers even if this one fails
			continue
		}
		g.capabilitiesMu.Unlock()

		activatedServers = append(activatedServers, serverName)
	}

	log.Log(fmt.Sprintf("- Successfully activated profile '%s' with %d server(s): %s", profileName, len(activatedServers), strings.Join(activatedServers, ", ")))
	if len(skippedServers) > 0 {
		log.Log(fmt.Sprintf("- Skipped %d already-active server(s): %s", len(skippedServers), strings.Join(skippedServers, ", ")))
	}

	return nil
}

// serverValidation holds validation results for a single server
type serverValidation struct {
	serverName     string
	missingSecrets []string
	missingConfig  []string
	imagePullError error
}

func activateProfileHandler(g *Gateway, _ *clientConfig) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse profile-id parameter
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

		profileName := strings.TrimSpace(params.Name)

		// Use the ActivateProfile method
		err = g.ActivateProfile(ctx, profileName)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error: %v", err),
				}},
				IsError: true,
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Successfully activated profile '%s'", profileName),
			}},
		}, nil
	}
}
