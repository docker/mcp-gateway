package server

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/policy"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
	policycontext "github.com/docker/mcp-gateway/pkg/policy/context"
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
	// Policy describes the policy decision for this server.
	Policy *policy.Decision `json:"policy,omitempty"`
}

func List(ctx context.Context, docker docker.Client, policyClient policy.Client, quiet bool) ([]ListEntry, error) {
	registry, _, err := loadRegistryWithConfig(ctx, docker)
	if err != nil {
		return nil, err
	}

	// Read the catalog to get server metadata (descriptions, etc.)
	catalogData, err := catalog.Get(ctx)
	if err != nil {
		return nil, err
	}
	catalogID := catalog.DockerCatalogFilename
	if _, name, _, err := catalog.ReadOne(ctx, catalog.DockerCatalogFilename); err == nil && name != "" {
		catalogID = name
	}
	policyCtx := policycontext.Context{
		Catalog:                  catalogID,
		WorkingSet:               "",
		ServerSourceTypeOverride: "registry",
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

	// Build batch request for all server policy evaluations.
	serverNames := registry.ServerNames()
	var requests []policy.Request
	serverIndices := make(map[string]int) // Map server name to request index.
	for _, serverName := range serverNames {
		if server, found := catalogData.Servers[serverName]; found {
			serverIndices[serverName] = len(requests)
			requests = append(requests, policycontext.BuildRequest(
				policyCtx,
				serverName,
				server,
				"",
				policy.ActionLoad,
			))
		}
	}

	// Evaluate all requests in a single batch call.
	var decisions []policy.Decision
	if policyClient != nil && len(requests) > 0 {
		decisions, err = policyClient.EvaluateBatch(ctx, requests)
		decisions, _ = policycli.NormalizeBatchDecisions(requests, decisions, err)
	}

	var entries []ListEntry
	for _, serverName := range serverNames {
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
			if idx, ok := serverIndices[serverName]; ok && idx < len(decisions) {
				entry.Policy = policy.DecisionForOutput(decisions[idx])
			}

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
				tile := registry.Servers[serverName]
				// Validate config requirements against user configuration
				entry.Config = validateConfigRequirements(server.Config, tile.Config)
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
