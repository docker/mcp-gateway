package gateway

import (
	"context"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/catalog"
)

// ServerSecretsInput represents the information needed to build secrets URIs for a server
type ServerSecretsInput struct {
	Secrets []catalog.Secret // Secret definitions from server catalog
	OAuth   *catalog.OAuth   // OAuth config (nil if no OAuth)
}

// BuildSecretsURIs generates se:// URIs for the given server inputs.
// It queries the Secrets Engine to verify which secrets actually exist,
// and for OAuth-enabled servers, prioritizes OAuth tokens over PATs.
func BuildSecretsURIs(ctx context.Context, inputs []ServerSecretsInput) map[string]string {
	uris := make(map[string]string)

	// Query Secrets Engine once to get all available secrets
	allSecrets, _ := secret.GetSecrets(ctx)
	secrets := make(map[string]string)
	for _, e := range allSecrets {
		secrets[e.ID] = string(e.Value)
	}

	for _, input := range inputs {
		// Server has no OAuth configured - use secret directly
		if input.OAuth == nil {
			for _, s := range input.Secrets {
				key := secret.GetDefaultSecretKey(s.Name)
				if val := secrets[key]; val != "" {
					uris[s.Name] = "se://" + key
				}
			}
			continue
		}

		// Server has OAuth - check OAuth token first, fall back to PAT
		secretToOAuthKey := make(map[string]string)
		for _, p := range input.OAuth.Providers {
			secretToOAuthKey[p.Secret] = secret.GetOAuthKey(p.Provider)
		}

		for _, s := range input.Secrets {
			// Try OAuth first
			if oauthKey, ok := secretToOAuthKey[s.Name]; ok {
				if _, exists := secrets[oauthKey]; exists {
					uris[s.Name] = "se://" + oauthKey
					continue
				}
			}
			// Fallback to PAT (must be non-empty)
			patKey := secret.GetDefaultSecretKey(s.Name)
			if val := secrets[patKey]; val != "" {
				uris[s.Name] = "se://" + patKey
			}
		}
	}
	return uris
}
