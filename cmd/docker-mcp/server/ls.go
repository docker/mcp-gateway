package server

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/docker"
)

type ConfigStatus string

const (
	ConfigStatusNone     ConfigStatus = "none"     // No configuration needed
	ConfigStatusRequired ConfigStatus = "required" // Configuration required but not set
	ConfigStatusPartial  ConfigStatus = "partial"  // Some configuration set but not all
	ConfigStatusDone     ConfigStatus = "done"     // All configuration complete
)

type ListEntry struct {
	Name        string
	Description string
	Secrets     ConfigStatus
	Config      ConfigStatus
}

func List(ctx context.Context, docker docker.Client) ([]ListEntry, error) {
	// Read the registry to get enabled servers
	registryYAML, err := config.ReadRegistry(ctx, docker)
	if err != nil {
		return nil, err
	}

	registry, err := config.ParseRegistryConfig(registryYAML)
	if err != nil {
		return nil, err
	}
	fmt.Println("registryYAML", string(registryYAML))
	fmt.Printf("DEBUG: Parsed registry: %+v\n", registry)

	// Read user's configuration to populate registry tiles
	userConfigYAML, err := config.ReadConfig(ctx, docker)
	if err != nil {
		return nil, fmt.Errorf("reading user config: %w", err)
	}

	userConfig, err := config.ParseConfig(userConfigYAML)
	if err != nil {
		return nil, fmt.Errorf("parsing user config: %w", err)
	}

	fmt.Printf("DEBUG: User config loaded: %+v\n", userConfig)

	// Populate registry tiles with user config
	for serverName, tile := range registry.Servers {
		if len(tile.Config) == 0 {
			if userServerConfig, hasUserConfig := userConfig[serverName]; hasUserConfig {
				tile.Config = userServerConfig
				registry.Servers[serverName] = tile
				fmt.Printf("DEBUG: Populated config for %s: %+v\n", serverName, userServerConfig)
			}
		}
	}

	// Read the catalog to get server metadata (descriptions, etc.)
	catalogData, err := catalog.Get(ctx)
	if err != nil {
		return nil, err
	}

	// Get the list of configured secrets
	configuredSecrets, err := desktop.NewSecretsClient().ListJfsSecrets(ctx)
	if err != nil {
		return nil, err
	}

	// Create a map of configured secret names for quick lookup
	configuredSecretNames := make(map[string]struct{})
	for _, secret := range configuredSecrets {
		// println(secret.Name)
		configuredSecretNames[secret.Name] = struct{}{}
	}

	isSecretConfigured := func(secret catalog.Secret) bool {
		_, ok := configuredSecretNames[secret.Name]
		return ok
	}

	// fmt.Println(configuredSecrets)
	// fmt.Println(configuredSecretNames)

	var entries []ListEntry
	for _, serverName := range registry.ServerNames() {
		entry := ListEntry{
			Name:        serverName,
			Description: "",
			Secrets:     ConfigStatusNone, // Default to no secrets needed
			Config:      ConfigStatusNone, // Default to no config needed
		}

		// Get description and check configuration from catalog
		if server, found := catalogData.Servers[serverName]; found {
			entry.Description = server.Description

			// Check secrets configuration
			if len(server.Secrets) > 0 {

				// Server requires secrets, check if they are configured
				// hasAllSecrets := true
				// hasSomeSecrets := false
				hasSomeSecrets := slices.ContainsFunc(server.Secrets, isSecretConfigured)
				hasAllSecrets := !slices.ContainsFunc(server.Secrets, func(s catalog.Secret) bool { return !isSecretConfigured(s) })

				// for _, secret := range server.Secrets {
				// 	if isSecretConfigured(secret.Name) {
				// 		hasSomeSecrets = true
				// 	} else {
				// 		hasAllSecrets = false
				// 	}
				// }

				switch {
				case hasAllSecrets:
					entry.Secrets = ConfigStatusDone
				case hasSomeSecrets:
					entry.Secrets = ConfigStatusPartial
				default:
					entry.Secrets = ConfigStatusRequired
				}

			}
			// Check other config requirements (non-OAuth config)
			if len(server.Config) > 0 {
				fmt.Println(len(server.Config))

				// Estoy esta retornando DONE para todos los configs solo por el OAuth
				// Check OAuth configuration

				// if server.IsOAuthServer() {
				// 	// Check if OAuth is configured
				// 	tile, hasConfig := registry.Servers[serverName]
				// 	if !hasConfig || len(tile.Config) == 0 {
				// 		entry.Config = ConfigStatusRequired
				// 	} else {
				// 		// Check if OAuth credentials are in config
				// 		if _, hasOAuth := tile.Config["oauth"]; hasOAuth {
				// 			entry.Config = ConfigStatusDone
				// 		} else {
				// 			entry.Config = ConfigStatusPartial
				// 		}
				// 	}
				// }

				// Server has config requirements beyond OAuth
				tile, hasConfig := registry.Servers[serverName]
				fmt.Println("tile", tile)
				fmt.Println("HAS OCNFIG", hasConfig)
				fmt.Println("tile config", len(tile.Config))
				if !hasConfig || len(tile.Config) == 0 {
					// if entry.Config == ConfigStatusNone {
					entry.Config = ConfigStatusRequired
					// }
				} else {
					// Validate config requirements against user configuration
					// configStatus := validateConfigRequirements(server.Config, tile.Config)
					entry.Config = validateConfigRequirements(server.Config, tile.Config)
					// if entry.Config == ConfigStatusNone {
					// 	entry.Config = configStatus
					// } else if configStatus == ConfigStatusRequired {
					// 	// If OAuth was done but other config is required, mark as partial
					// 	entry.Config = ConfigStatusPartial
					// }
				}
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// DisplayString returns the display string for a ConfigStatus
func (cs ConfigStatus) DisplayString() string {
	switch cs {
	case ConfigStatusNone:
		return "-"
	case ConfigStatusRequired:
		return "▲ required"
	case ConfigStatusPartial:
		return "◐ partial"
	case ConfigStatusDone:
		return "✓ done"
	default:
		return "-"
	}
}

// validateConfigRequirements validates user configuration against server requirements
func validateConfigRequirements(requirements []any, userConfig map[string]any) ConfigStatus {
	requirementsJSON, _ := json.MarshalIndent(requirements, "", "  ")
	fmt.Printf("requirements ---\n%s\n", string(requirementsJSON))

	// Flatten requirements into dot-notation keys for easy comparison
	flatReq := collectRequiredFields(requirements)
	var flatReqKeys []string
	for k := range flatReq {
		flatReqKeys = append(flatReqKeys, k)
	}
	slices.Sort(flatReqKeys)
	flatReqJSON, _ := json.MarshalIndent(flatReqKeys, "", "  ")
	fmt.Printf("requirements (flattened) ---\n%s\n", string(flatReqJSON))

	// Flatten user config for easier comparison (dot notation like a.b)
	flattened := flattenMap("", userConfig)
	userConfigJSON, _ := json.MarshalIndent(flattened, "", "  ")
	fmt.Printf("userConfig (flattened) ---\n%s\n", string(userConfigJSON))

	// Determine status based on total vs configured counts across ALL requirements
	total := len(flatReq)
	if total == 0 {
		return ConfigStatusDone
	}
	configured := 0
	for k := range flatReq {
		if _, ok := flattened[k]; ok {
			configured++
		}
	}
	fmt.Printf("TOTAL required keys: %d, CONFIGURED: %d\n", total, configured)
	switch {
	case configured == 0:
		return ConfigStatusRequired
	case configured < total:
		return ConfigStatusPartial
	default:
		return ConfigStatusDone
	}
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
