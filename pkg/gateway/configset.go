package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oci"
)

type configValue struct {
	Server string         `json:"server"`
	Config map[string]any `json:"config"`
}

func configSetHandler(g *Gateway) mcp.ToolHandler {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		if params.Config == nil {
			return nil, fmt.Errorf("config parameter is required")
		}

		serverName := strings.TrimSpace(params.Server)
		canonicalServerName := oci.CanonicalizeServerName(serverName)

		// Check if server exists in catalog
		serverConfig, _, serverExists := g.configuration.Find(serverName)

		if !serverExists {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error: Server '%s' not found in catalog. Use mcp-find to search for available servers.", serverName),
				}},
			}, nil
		}

		// Validate config against server's schema if schema exists
		if serverConfig != nil && len(serverConfig.Spec.Config) > 0 {
			var validationErrors []string
			var schemaInfo strings.Builder

			schemaInfo.WriteString("Server config schema:\n")

			for _, configItem := range serverConfig.Spec.Config {
				// Config items should be schema objects
				schemaMap, ok := configItem.(map[string]any)
				if !ok {
					continue
				}

				// Get the name field - this identifies which config to validate
				configName, ok := schemaMap["name"].(string)
				if !ok || configName == "" {
					continue
				}

				// Add schema to info
				schemaBytes, _ := json.MarshalIndent(schemaMap, "  ", "  ")
				schemaInfo.WriteString(fmt.Sprintf("\n%s:\n  %s\n", configName, string(schemaBytes)))

				// Convert the schema map to a jsonschema.Schema for validation
				schemaBytes, err := json.Marshal(schemaMap)
				if err != nil {
					validationErrors = append(validationErrors, fmt.Sprintf("%s: invalid schema definition", configName))
					continue
				}

				var schema jsonschema.Schema
				if err := json.Unmarshal(schemaBytes, &schema); err != nil {
					validationErrors = append(validationErrors, fmt.Sprintf("%s: invalid schema definition", configName))
					continue
				}

				// Resolve the schema
				resolved, err := schema.Resolve(nil)
				if err != nil {
					validationErrors = append(validationErrors, fmt.Sprintf("%s: schema resolution failed", configName))
					continue
				}

				// Validate the config value against the schema
				if err := resolved.Validate(params.Config); err != nil {
					validationErrors = append(validationErrors, fmt.Sprintf("%s: %s", configName, err.Error()))
				}
			}

			// If validation failed, return error with schema
			if len(validationErrors) > 0 {
				errorMessage := fmt.Sprintf("Config validation failed for server '%s':\n\n", serverName)
				for _, errMsg := range validationErrors {
					errorMessage += fmt.Sprintf("  - %s\n", errMsg)
				}
				errorMessage += "\n" + schemaInfo.String()

				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: errorMessage,
					}},
					IsError: true,
				}, nil
			}
		}

		// Store old config for comparison
		oldConfig := g.configuration.config[canonicalServerName]
		oldConfigJSON, _ := json.MarshalIndent(oldConfig, "", "  ")

		// Set the configuration
		g.configuration.config[canonicalServerName] = params.Config

		// Format new config for display
		newConfigJSON, _ := json.MarshalIndent(params.Config, "", "  ")

		// Log the configuration change
		log.Log(fmt.Sprintf("  - Set config for server '%s': %s", serverName, string(newConfigJSON)))

		var resultMessage string
		if oldConfig != nil {
			resultMessage = fmt.Sprintf("Successfully updated config for server '%s':\n\nOld config:\n%s\n\nNew config:\n%s",
				serverName, string(oldConfigJSON), string(newConfigJSON))
		} else {
			resultMessage = fmt.Sprintf("Successfully set config for server '%s':\n\n%s", serverName, string(newConfigJSON))
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: resultMessage,
			}},
		}, nil
	}
}
