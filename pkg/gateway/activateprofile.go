package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/gateway/project"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

var errProfileNotFound = errors.New("profile not found")

// loadProfileFromProject attempts to load a profile from the project's profiles.json
// Returns the WorkingSet if found, or errProfileNotFound if not found
func loadProfileFromProject(ctx context.Context, profileName string) (*workingset.WorkingSet, error) {
	profiles, err := project.LoadProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load profiles.json: %w", err)
	}

	if profile, found := profiles[profileName]; found {
		log.Log(fmt.Sprintf("- Found profile '%s' in project's profiles.json", profileName))
		return &profile, nil
	}

	return nil, errProfileNotFound
}

// convertWorkingSetToConfiguration converts a WorkingSet to a Configuration object
func (g *Gateway) convertWorkingSetToConfiguration(ctx context.Context, ws workingset.WorkingSet) (Configuration, error) {
	// Ensure snapshots are resolved
	ociService := oci.NewService()
	if err := ws.EnsureSnapshotsResolved(ctx, ociService); err != nil {
		return Configuration{}, fmt.Errorf("failed to resolve snapshots: %w", err)
	}

	// Build configuration similar to WorkingSetConfiguration.readOnce
	cfg := make(map[string]map[string]any)
	configs := make([]ServerSecretConfig, 0, len(ws.Servers))
	toolsConfig := config.ToolsConfig{ServerTools: make(map[string][]string)}
	serverNames := make([]string, 0)
	servers := make(map[string]catalog.Server)

	for _, server := range ws.Servers {
		// Skip non-image/remote/registry servers
		if server.Type != workingset.ServerTypeImage &&
			server.Type != workingset.ServerTypeRemote &&
			server.Type != workingset.ServerTypeRegistry {
			continue
		}

		serverName := server.Snapshot.Server.Name
		servers[serverName] = server.Snapshot.Server
		serverNames = append(serverNames, serverName)
		cfg[serverName] = server.Config

		// Build secrets configs
		namespace := ""
		configs = append(configs, ServerSecretConfig{
			Secrets:   server.Snapshot.Server.Secrets,
			OAuth:     server.Snapshot.Server.OAuth,
			Namespace: namespace,
		})

		// Add tools
		if server.Tools != nil {
			toolsConfig.ServerTools[serverName] = server.Tools
		}
	}

	secrets := BuildSecretsURIs(ctx, configs)

	return Configuration{
		serverNames: serverNames,
		servers:     servers,
		config:      cfg,
		tools:       toolsConfig,
		secrets:     secrets,
	}, nil
}

// ActivateProfile activates a profile by merging its servers into the gateway
// The WorkingSet should be loaded by the caller before calling this method
func (g *Gateway) ActivateProfile(ctx context.Context, ws workingset.WorkingSet) error {
	log.Log(fmt.Sprintf("- Activating profile '%s'", ws.Name))

	// Convert WorkingSet to Configuration
	profileConfig, err := g.convertWorkingSetToConfiguration(ctx, ws)
	if err != nil {
		return fmt.Errorf("failed to convert profile '%s': %w", ws.Name, err)
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
			log.Log(fmt.Sprintf("- All servers from profile '%s' are already active: %s", ws.Name, strings.Join(skippedServers, ", ")))
		} else {
			log.Log(fmt.Sprintf("- No new servers to activate from profile '%s'", ws.Name))
		}
		return nil
	}

	// Validate ALL servers before activating any
	// Note: Validation ensures prerequisites (secrets, config, images) are met.
	// Actual capability loading happens during activation and may partially succeed.
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
				// Config items are object schemas with a "properties" map.
				// The "name" field is just an identifier, not a key in serverConfigMap.
				schemaMap, ok := configItem.(map[string]any)
				if !ok {
					continue
				}

				properties, ok := schemaMap["properties"].(map[string]any)
				if !ok {
					continue
				}

				// Build a set of required property names
				requiredProps := make(map[string]bool)
				if requiredList, ok := schemaMap["required"].([]any); ok {
					for _, r := range requiredList {
						if s, ok := r.(string); ok {
							requiredProps[s] = true
						}
					}
				}

				// Validate each property individually
				for propName, propSchema := range properties {
					propSchemaMap, ok := propSchema.(map[string]any)
					if !ok {
						continue
					}

					// Get the value from the user-provided config
					configValue, exists := serverConfigMap[propName]
					if !exists {
						// If the property has a default, the server will use it
						if _, hasDefault := propSchemaMap["default"]; hasDefault {
							continue
						}
						// Only flag as missing if explicitly required
						if requiredProps[propName] {
							validation.missingConfig = append(validation.missingConfig, fmt.Sprintf("%s (missing)", propName))
						}
						continue
					}

					// Validate the value against the property schema
					schemaBytes, err := json.Marshal(propSchemaMap)
					if err != nil {
						validation.missingConfig = append(validation.missingConfig, fmt.Sprintf("%s (invalid schema)", propName))
						continue
					}

					var propSchemaObj jsonschema.Schema
					if err := json.Unmarshal(schemaBytes, &propSchemaObj); err != nil {
						validation.missingConfig = append(validation.missingConfig, fmt.Sprintf("%s (invalid schema)", propName))
						continue
					}

					resolved, err := propSchemaObj.Resolve(nil)
					if err != nil {
						validation.missingConfig = append(validation.missingConfig, fmt.Sprintf("%s (schema resolution failed)", propName))
						continue
					}

					if err := resolved.Validate(configValue); err != nil {
						errMsg := err.Error()
						if len(errMsg) > 100 {
							errMsg = errMsg[:97] + "..."
						}
						validation.missingConfig = append(validation.missingConfig, fmt.Sprintf("%s (%s)", propName, errMsg))
					}
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
		errorMessages = append(errorMessages, fmt.Sprintf("Cannot activate profile '%s'. Validation failed for %d server(s):", ws.Name, len(validationErrors)))

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
	// Acquire configuration mutex to ensure atomic updates
	g.configurationMu.Lock()
	defer g.configurationMu.Unlock()

	var activatedServers []string
	var failedServers []string

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
			failedServers = append(failedServers, serverName)
			// Continue with other servers even if this one fails
			continue
		}

		// Update g.mcpServer with the new capabilities
		g.capabilitiesMu.Lock()
		newCaps := g.allCapabilities(serverName)
		if err := g.updateServerCapabilities(serverName, oldCaps, newCaps, nil); err != nil {
			g.capabilitiesMu.Unlock()
			log.Log(fmt.Sprintf("Warning: Failed to update server capabilities for '%s': %v", serverName, err))
			failedServers = append(failedServers, serverName)
			// Continue with other servers even if this one fails
			continue
		}
		g.capabilitiesMu.Unlock()

		activatedServers = append(activatedServers, serverName)
	}

	// Log results
	if len(activatedServers) > 0 {
		log.Log(fmt.Sprintf("- Successfully activated profile '%s' with %d server(s): %s", ws.Name, len(activatedServers), strings.Join(activatedServers, ", ")))
	}
	if len(skippedServers) > 0 {
		log.Log(fmt.Sprintf("- Skipped %d already-active server(s): %s", len(skippedServers), strings.Join(skippedServers, ", ")))
	}
	if len(failedServers) > 0 {
		log.Log(fmt.Sprintf("- Failed to activate %d server(s): %s", len(failedServers), strings.Join(failedServers, ", ")))
		// Return error if all servers failed to activate
		if len(activatedServers) == 0 {
			return fmt.Errorf("failed to activate any servers from profile '%s'", ws.Name)
		}
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

		// Load the profile from either profiles.json or database
		var ws *workingset.WorkingSet

		// First, try to load from project's profiles.json
		projectProfile, err := loadProfileFromProject(ctx, profileName)
		if err != nil && !errors.Is(err, errProfileNotFound) {
			log.Log(fmt.Sprintf("Warning: Failed to check project profiles: %v", err))
		}

		if projectProfile != nil {
			// Found in project's profiles.json
			log.Log(fmt.Sprintf("- Found profile '%s' in project's profiles.json", profileName))
			ws = projectProfile
		} else {
			// Not found in project, try database
			log.Log(fmt.Sprintf("- Profile '%s' not found in project's profiles.json, checking database", profileName))

			dao, err := db.New()
			if err != nil {
				return nil, fmt.Errorf("failed to create database client: %w", err)
			}
			defer dao.Close()

			dbProfile, err := dao.GetWorkingSet(ctx, profileName)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return &mcp.CallToolResult{
						Content: []mcp.Content{&mcp.TextContent{
							Text: fmt.Sprintf("Error: Profile '%s' not found in project or database", profileName),
						}},
						IsError: true,
					}, nil
				}
				return nil, fmt.Errorf("failed to load profile from database: %w", err)
			}

			log.Log(fmt.Sprintf("- Found profile '%s' in database", profileName))
			wsFromDb := workingset.NewFromDb(dbProfile)
			ws = &wsFromDb
		}

		// Activate the profile
		err = g.ActivateProfile(ctx, *ws)
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
				Text: fmt.Sprintf("Successfully activated profile '%s'", ws.Name),
			}},
		}, nil
	}
}
