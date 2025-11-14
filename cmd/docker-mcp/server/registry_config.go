package server

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/docker"
)

// loadRegistryWithConfig reads the registry and populates it with user config values.
// It returns the populated registry and the userConfig map for further use.
// IMPORTANT: config.yaml is the single source of truth for all config values.
// We completely ignore any Config values that may exist in registry.yaml.
func loadRegistryWithConfig(ctx context.Context, docker docker.Client) (config.Registry, map[string]map[string]any, error) {
	// Read registry.yaml that contains which servers are enabled.
	// NOTE: We only use this to determine which servers are enabled (the map keys).
	// We completely ignore any Config values in the Tile structs from registry.yaml.
	registryYAML, err := config.ReadRegistry(ctx, docker)
	if err != nil {
		return config.Registry{}, nil, fmt.Errorf("reading registry config: %w", err)
	}

	registry, err := config.ParseRegistryConfig(registryYAML)
	if err != nil {
		return config.Registry{}, nil, fmt.Errorf("parsing registry config: %w", err)
	}

	// Read user's configuration from config.yaml (the single source of truth)
	userConfigYAML, err := config.ReadConfig(ctx, docker)
	if err != nil {
		return config.Registry{}, nil, fmt.Errorf("reading user config: %w", err)
	}

	userConfig, err := config.ParseConfig(userConfigYAML)
	if err != nil {
		return config.Registry{}, nil, fmt.Errorf("parsing user config: %w", err)
	}

	// Ensure userConfig is never nil (empty config file results in nil map)
	if userConfig == nil {
		userConfig = make(map[string]map[string]any)
	}

	// Populate registry tiles with configs from config.yaml ONLY
	// We completely ignore any Config values that were parsed from registry.yaml
	for serverName, tile := range registry.Servers {
		// Only populate from config.yaml (the single source of truth)
		// We ignore any Config values that may have been parsed from registry.yaml
		if userServerConfig, hasUserConfig := userConfig[serverName]; hasUserConfig {
			tile.Config = userServerConfig
		} else {
			tile.Config = nil
		}

		registry.Servers[serverName] = tile
	}

	return registry, userConfig, nil
}

// configField represents a missing config field that needs to be prompted
type configField struct {
	Key         string
	Description string
	Type        string
	Default     any
	Enum        []any
	Format      string
}

// skipConfigValue is a sentinel value indicating the user wants to skip this config field
var skipConfigValue = struct{}{}

// getMissingConfigs returns a list of config fields that are required but not yet configured
func getMissingConfigs(configSchema []any, userConfig map[string]any) []configField {
	// Collect all required fields from the schema
	requiredFields := collectRequiredFields(configSchema)

	// Flatten user config for comparison
	flattened := flattenMap("", userConfig)

	// Build a map of property metadata for each field
	propertyMap := buildPropertyMap(configSchema)

	var missing []configField
	for key := range requiredFields {
		// Check if this field is already configured with a non-empty value
		if value, ok := flattened[key]; ok {
			// Skip if the value is not empty
			if !isEmptyValue(value) {
				continue
			}
			// If the value is empty, treat it as missing and prompt for it
		}

		// Get property metadata
		prop, ok := propertyMap[key]
		if !ok {
			// If we can't find the property, still add it with minimal info
			missing = append(missing, configField{
				Key: key,
			})
			continue
		}

		missing = append(missing, prop)
	}

	// Sort by key for consistent ordering
	slices.SortFunc(missing, func(a, b configField) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})

	return missing
}

// parseRequiredFields extracts all required field names from the config schema
func parseRequiredFields(req map[string]any) map[string]bool {
	fields := make(map[string]bool)
	// Only consider declared properties; ignore the top-level "name" field which
	// identifies the server and should not be treated as a required config key.
	if properties, ok := req["properties"].(map[string]any); ok {
		walkProperties("", properties, fields)
	}
	return fields
}

// walkProperties recursively collects leaf property keys into dot-notation
func walkProperties(prefix string, properties map[string]any, out map[string]bool) {
	for propName, propDef := range properties {
		// compute key with prefix
		key := propName
		if prefix != "" {
			key = prefix + "." + propName
		}
		// If this property itself has nested properties, recurse
		if propDefMap, ok := propDef.(map[string]any); ok {
			if nestedProps, hasNested := propDefMap["properties"].(map[string]any); hasNested {
				walkProperties(key, nestedProps, out)
				continue
			}
		}
		// Otherwise it's a leaf
		out[key] = true
	}
}

// collectRequiredFields merges required fields from a list of requirement objects
func collectRequiredFields(requirements []any) map[string]bool {
	out := make(map[string]bool)
	for _, r := range requirements {
		reqMap, ok := r.(map[string]any)
		if !ok {
			continue
		}
		for k := range parseRequiredFields(reqMap) {
			out[k] = true
		}
	}
	return out
}

// buildPropertyMap builds a map of dot-notation keys to property metadata
func buildPropertyMap(configSchema []any) map[string]configField {
	result := make(map[string]configField)

	for _, schemaItem := range configSchema {
		schemaMap, ok := schemaItem.(map[string]any)
		if !ok {
			continue
		}

		properties, ok := schemaMap["properties"].(map[string]any)
		if !ok {
			continue
		}

		walkPropertiesForMetadata("", properties, result)
	}

	return result
}

// recursively walks properties and builds metadata map
func walkPropertiesForMetadata(prefix string, properties map[string]any, out map[string]configField) {
	for propName, propDef := range properties {
		key := propName
		if prefix != "" {
			key = prefix + "." + propName
		}

		propMap, ok := propDef.(map[string]any)
		if !ok {
			continue
		}

		// Check if this property has nested properties
		if nestedProps, hasNested := propMap["properties"].(map[string]any); hasNested {
			walkPropertiesForMetadata(key, nestedProps, out)
			continue
		}

		// This is a leaf property, extract metadata
		field := configField{
			Key: key,
		}

		if desc, ok := propMap["description"].(string); ok {
			field.Description = desc
		}

		if typ, ok := propMap["type"].(string); ok {
			field.Type = typ
		}

		if def, ok := propMap["default"]; ok {
			field.Default = def
		}

		if enum, ok := propMap["enum"].([]any); ok {
			field.Enum = enum
		}

		if format, ok := propMap["format"].(string); ok {
			field.Format = format
		}

		out[key] = field
	}
}

// setNestedConfig sets a value in a nested map using dot-notation key (e.g., "a.b.c" => map["a"]["b"]["c"])
func setNestedConfig(config map[string]any, key string, value any) {
	parts := strings.Split(key, ".")

	current := config
	for i := range len(parts) - 1 {
		part := parts[i]
		if _, ok := current[part]; !ok {
			current[part] = make(map[string]any)
		}

		next, ok := current[part].(map[string]any)
		if !ok {
			// If it's not a map, we need to replace it
			current[part] = make(map[string]any)
			next = current[part].(map[string]any)
		}
		current = next
	}

	// Set the final value
	current[parts[len(parts)-1]] = value
}

// deepCopyMap creates a deep copy of a map[string]any
func deepCopyMap(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		if subMap, ok := v.(map[string]any); ok {
			result[k] = deepCopyMap(subMap)
		} else {
			result[k] = v
		}
	}
	return result
}

// isEmptyValue checks if a value is considered "empty" and should be skipped
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	// Check for sentinel skip value
	if v == skipConfigValue {
		return true
	}
	if s, ok := v.(string); ok {
		return s == ""
	}
	// Check for empty maps
	if m, ok := v.(map[string]any); ok {
		return len(m) == 0
	}
	return false
}

// flattenMap flattens a nested map (with map[string]any values) into dot-notation keys
// Example: {"confluence": {"url": "x"}} => {"confluence.url": "x"}
func flattenMap(prefix string, m map[string]any) map[string]any {
	out := make(map[string]any)
	join := func(a, b string) string {
		if a == "" {
			return b
		}
		return a + "." + b
	}
	for k, v := range m {
		key := join(prefix, k)
		if sub, ok := v.(map[string]any); ok {
			for fk, fv := range flattenMap(key, sub) {
				out[fk] = fv
			}
			continue
		}
		out[key] = v
	}
	return out
}

// validateConfigRequirements validates user configuration against server requirements
func validateConfigRequirements(requirements []any, userConfig map[string]any) ConfigStatus {
	// If userConfig is empty, check if config is required
	if len(userConfig) == 0 {
		// Check if there are any requirements first
		flatReq := collectRequiredFields(requirements)
		if len(flatReq) == 0 {
			return ConfigStatusDone
		}
		return ConfigStatusRequired
	}

	flatReq := collectRequiredFields(requirements)
	var flatReqKeys []string
	for k := range flatReq {
		flatReqKeys = append(flatReqKeys, k)
	}
	slices.Sort(flatReqKeys)

	// Flatten user config for easier comparison (dot notation like a.b)
	flattened := flattenMap("", userConfig)

	// Determine status based on total vs configured counts across ALL requirements
	total := len(flatReq)
	if total == 0 {
		return ConfigStatusDone
	}
	configured := 0
	for k := range flatReq {
		if value, ok := flattened[k]; ok {
			// Only count as configured if the value is non-empty
			if !isEmptyValue(value) {
				configured++
			}
		}
	}

	switch {
	case configured == 0:
		return ConfigStatusRequired
	case configured < total:
		return ConfigStatusPartial
	default:
		return ConfigStatusDone
	}
}
