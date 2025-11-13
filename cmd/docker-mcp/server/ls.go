package server

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/docker/mcp-gateway/pkg/catalog"
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
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Secrets     ConfigStatus `json:"secrets"`
	Config      ConfigStatus `json:"config"`
	OAuth       ConfigStatus `json:"oauth"`
}

func List(ctx context.Context, docker docker.Client, quiet bool) ([]ListEntry, error) {
	registry, _, err := loadRegistryWithConfig(ctx, docker)
	if err != nil {
		return nil, err
	}

	// Read the catalog to get server metadata (descriptions, etc.)
	catalogData, err := catalog.Get(ctx)
	if err != nil {
		return nil, err
	}

	// Get the map of configured secret names
	configuredSecretNames, err := getConfiguredSecretNames(ctx)
	if err != nil {
		// If we can't get secrets, assume none are configured
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: error fetching secrets: %v\n", err)
		}
		configuredSecretNames = make(map[string]struct{})
	}

	isSecretConfigured := func(secret catalog.Secret) bool {
		_, ok := configuredSecretNames[secret.Name]
		return ok
	}

	// Get OAuth apps to check authorization status
	authClient := desktop.NewAuthClient()
	oauthApps, err := authClient.ListOAuthApps(ctx)
	if err != nil {
		// If we can't get OAuth apps, we'll just skip OAuth checking
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: error fetching OAuth apps: %v\n", err)
		}
	}

	var entries []ListEntry
	for _, serverName := range registry.ServerNames() {
		entry := ListEntry{
			Name:        serverName,
			Description: "",
			Secrets:     ConfigStatusNone, // Default to no secrets needed
			Config:      ConfigStatusNone, // Default to no config needed
			OAuth:       ConfigStatusNone, // Default to no OAuth needed
		}

		// Get description and check configuration from catalog
		if server, found := catalogData.Servers[serverName]; found {
			entry.Description = server.Description

			// Check secrets configuration
			if len(server.Secrets) > 0 {
				hasSomeSecrets := slices.ContainsFunc(server.Secrets, isSecretConfigured)
				hasAllSecrets := !slices.ContainsFunc(server.Secrets, func(s catalog.Secret) bool { return !isSecretConfigured(s) })

				switch {
				case hasAllSecrets:
					entry.Secrets = ConfigStatusDone
				case hasSomeSecrets:
					entry.Secrets = ConfigStatusPartial
				default:
					entry.Secrets = ConfigStatusRequired
				}

			}

			// Check OAuth configuration
			if server.IsOAuthServer() {
				providerName := server.OAuth.Providers[0].Provider

				// Find OAuth app by provider name
				var oauthApp *desktop.OAuthApp
				for i := range oauthApps {
					if oauthApps[i].App == providerName {
						oauthApp = &oauthApps[i]
						break
					}
				}

				// Also check if server name matches (for DCR servers)
				if oauthApp == nil {
					for i := range oauthApps {
						if oauthApps[i].App == serverName {
							oauthApp = &oauthApps[i]
							break
						}
					}
				}

				if oauthApp != nil && oauthApp.Authorized {
					entry.OAuth = ConfigStatusDone
				} else {
					entry.OAuth = ConfigStatusRequired
				}
			}

			// Check other config requirements (non-OAuth config)
			if len(server.Config) > 0 {

				tile, hasConfig := registry.Servers[serverName]
				if !hasConfig || len(tile.Config) == 0 {
					entry.Config = ConfigStatusRequired
				} else {
					// Validate config requirements against user configuration
					entry.Config = validateConfigRequirements(server.Config, tile.Config)
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
