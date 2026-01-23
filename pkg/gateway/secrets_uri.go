package gateway

import (
	"context"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/catalog"
)

// ServerSecretsInput represents info needed to build secrets URIs
type ServerSecretsInput struct {
	Secrets        []catalog.Secret // Secret definitions from server catalog
	OAuth          *catalog.OAuth   // OAuth config (nil if no OAuth)
	ProviderPrefix string           // Optional prefix for map keys (WorkingSet namespacing)
}

// BuildSecretsURIs generates se:// URIs for the given server inputs.
// Calls GetSecrets() to check availability and handles OAuth priority.
func BuildSecretsURIs(ctx context.Context, inputs []ServerSecretsInput) map[string]string {
	uris := make(map[string]string)

	// Get all available secrets
	allSecrets, _ := secret.GetSecrets(ctx)
	secretsMap := make(map[string]string)
	for _, e := range allSecrets {
		secretsMap[e.ID] = string(e.Value)
	}

	for _, input := range inputs {
		// Non-OAuth servers: use default key directly
		if input.OAuth == nil {
			for _, s := range input.Secrets {
				key := secret.GetDefaultSecretKey(s.Name)
				if secretsMap[key] != "" {
					mapKey := input.ProviderPrefix + s.Name
					uris[mapKey] = "se://" + key
				}
			}
			continue
		}

		// OAuth servers: check OAuth key first, fall back to PAT
		secretToOAuthKey := make(map[string]string)
		for _, p := range input.OAuth.Providers {
			secretToOAuthKey[p.Secret] = secret.GetOAuthKey(p.Provider)
		}

		for _, s := range input.Secrets {
			mapKey := input.ProviderPrefix + s.Name

			// Try OAuth first
			if oauthKey, ok := secretToOAuthKey[s.Name]; ok {
				if secretsMap[oauthKey] != "" {
					uris[mapKey] = "se://" + oauthKey
					continue
				}
			}
			// Fallback to PAT
			patKey := secret.GetDefaultSecretKey(s.Name)
			if secretsMap[patKey] != "" {
				uris[mapKey] = "se://" + patKey
			}
		}
	}
	return uris
}
