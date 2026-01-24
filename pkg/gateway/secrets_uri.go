package gateway

import (
	"context"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/catalog"
)

// ServerSecretConfig contains the secret definitions and OAuth config for a server.
type ServerSecretConfig struct {
	Secrets   []catalog.Secret // Secret definitions from server catalog
	OAuth     *catalog.OAuth   // OAuth config (if present, OAuth takes priority over PAT)
	Namespace string           // Prefix for map keys (used by WorkingSet for namespacing)
}

// BuildSecretsURIs generates se:// URIs for secrets with OAuth priority handling.
//
// For servers WITH OAuth configured:
//   - Check if OAuth token exists (docker/mcp/oauth/{provider})
//   - If yes, use OAuth URI
//   - If no, fall back to PAT URI (docker/mcp/{secret_name})
//
// For servers WITHOUT OAuth:
//   - Use PAT URI directly if secret exists
//
// Docker Desktop resolves se:// URIs at container runtime.
// Remote servers fetch actual values via remote.go.
func BuildSecretsURIs(ctx context.Context, configs []ServerSecretConfig) map[string]string {
	// secretNameToURI maps secret names to their se:// URIs
	// Key: secret name (e.g., "github.token" or "default_github.token" with namespace)
	// Value: se:// URI (e.g., "se://docker/mcp/github.token")
	secretNameToURI := make(map[string]string)

	// availableSecrets maps secret IDs to their values (used to check existence)
	allSecrets, _ := secret.GetSecrets(ctx)
	availableSecrets := make(map[string]string)
	for _, envelope := range allSecrets {
		availableSecrets[envelope.ID] = string(envelope.Value)
	}

	for _, cfg := range configs {
		// Server has no OAuth configured - use secret directly
		if cfg.OAuth == nil {
			for _, s := range cfg.Secrets {
				secretID := secret.GetDefaultSecretKey(s.Name)
				if availableSecrets[secretID] != "" {
					secretName := cfg.Namespace + s.Name
					secretNameToURI[secretName] = "se://" + secretID
				}
			}
			continue
		}

		// Build mapping from secret name to OAuth storage key
		secretToOAuthID := make(map[string]string)
		for _, p := range cfg.OAuth.Providers {
			secretToOAuthID[p.Secret] = secret.GetOAuthKey(p.Provider)
		}

		for _, s := range cfg.Secrets {
			secretName := cfg.Namespace + s.Name

			// Try OAuth first
			if oauthSecretID, ok := secretToOAuthID[s.Name]; ok {
				if availableSecrets[oauthSecretID] != "" {
					secretNameToURI[secretName] = "se://" + oauthSecretID
					continue
				}
			}
			// Fall back to PAT if it exists
			patSecretID := secret.GetDefaultSecretKey(s.Name)
			if availableSecrets[patSecretID] != "" {
				secretNameToURI[secretName] = "se://" + patSecretID
			}
		}
	}
	return secretNameToURI
}
