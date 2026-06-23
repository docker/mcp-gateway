package gateway

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

// serverValidation holds validation results for a single server.
type serverValidation struct {
	serverName     string
	missingSecrets []string
	missingConfig  []string
	imagePullError error
}

func validateWorkingSetServerConfigs(ws workingset.WorkingSet) []serverValidation {
	var validationErrors []serverValidation

	for _, server := range ws.Servers {
		if server.Snapshot == nil {
			continue
		}

		serverName := server.Snapshot.Server.Name
		missingConfig := validateServerConfig(server.Snapshot.Server, server.Config)
		if len(missingConfig) == 0 {
			continue
		}

		validationErrors = append(validationErrors, serverValidation{
			serverName:    serverName,
			missingConfig: missingConfig,
		})
	}

	return validationErrors
}

func validateServerConfig(server catalog.Server, serverConfigMap map[string]any) []string {
	var missingConfig []string

	for _, configItem := range server.Config {
		schemaMap, ok := configItem.(map[string]any)
		if !ok {
			continue
		}

		properties, ok := schemaMap["properties"].(map[string]any)
		if !ok {
			continue
		}

		requiredProps := requiredConfigProperties(schemaMap)

		for propName, propSchema := range properties {
			propSchemaMap, ok := propSchema.(map[string]any)
			if !ok {
				continue
			}

			configValue, exists := serverConfigMap[propName]
			if !exists {
				if _, hasDefault := propSchemaMap["default"]; hasDefault {
					continue
				}
				if requiredProps[propName] {
					missingConfig = append(missingConfig, fmt.Sprintf("%s (missing)", propName))
				}
				continue
			}
			if isEmptyConfigValue(configValue) {
				if requiredProps[propName] {
					missingConfig = append(missingConfig, fmt.Sprintf("%s (missing)", propName))
				}
				continue
			}

			schemaBytes, err := json.Marshal(propSchemaMap)
			if err != nil {
				missingConfig = append(missingConfig, fmt.Sprintf("%s (invalid schema)", propName))
				continue
			}

			var propSchemaObj jsonschema.Schema
			if err := json.Unmarshal(schemaBytes, &propSchemaObj); err != nil {
				missingConfig = append(missingConfig, fmt.Sprintf("%s (invalid schema)", propName))
				continue
			}

			resolved, err := propSchemaObj.Resolve(nil)
			if err != nil {
				missingConfig = append(missingConfig, fmt.Sprintf("%s (schema resolution failed)", propName))
				continue
			}

			if err := resolved.Validate(configValue); err != nil {
				errMsg := err.Error()
				if len(errMsg) > 100 {
					errMsg = errMsg[:97] + "..."
				}
				missingConfig = append(missingConfig, fmt.Sprintf("%s (%s)", propName, errMsg))
			}
		}
	}

	return missingConfig
}

func requiredConfigProperties(schemaMap map[string]any) map[string]bool {
	requiredProps := make(map[string]bool)
	switch requiredList := schemaMap["required"].(type) {
	case []any:
		for _, r := range requiredList {
			if s, ok := r.(string); ok {
				requiredProps[s] = true
			}
		}
	case []string:
		for _, s := range requiredList {
			requiredProps[s] = true
		}
	}
	return requiredProps
}

func isEmptyConfigValue(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return s == ""
	}
	if m, ok := v.(map[string]any); ok {
		return len(m) == 0
	}
	return false
}

func formatProfileValidationError(profileName string, validationErrors []serverValidation) error {
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
