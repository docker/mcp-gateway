package gateway

import (
	"context"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/log"
)

// ServerSecretConfig contains the secret definitions and OAuth config for a server.
type ServerSecretConfig struct {
	Secrets   []catalog.Secret // Secret definitions from server catalog
	OAuth     *catalog.OAuth   // OAuth config (if present, OAuth takes priority over direct secret)
	Namespace string           // Prefix for map keys (used by WorkingSet for namespacing)
}

// BuildSecretsURIs generates se:// URIs for secrets with OAuth priority handling.
//
// When the secrets engine is reachable, URIs are only generated for secrets that
// actually exist in the store (OAuth token checked first, then direct secret).
//
// When the secrets engine is unreachable (e.g. MSIX-sandboxed clients on Windows
// cannot follow AF_UNIX reparse points), URIs are generated for all declared secrets
// since we cannot check existence. Docker Desktop resolves se:// URIs at container
// runtime via named pipes, which are unaffected by MSIX restrictions.
func BuildSecretsURIs(ctx context.Context, configs []ServerSecretConfig) map[string]string {
	allSecrets, err := secret.GetSecrets(ctx)
	if err != nil {
		log.Logf("Warning: Failed to fetch secrets from secrets engine: %v", err)
		return buildFallbackURIs(configs)
	}

	availableSecrets := make(map[string]string)
	for _, envelope := range allSecrets {
		availableSecrets[envelope.ID] = string(envelope.Value)
	}
	return buildVerifiedURIs(configs, availableSecrets)
}

// buildFallbackURIs generates se:// URIs for all declared secrets without checking
// existence. Used when the secrets engine is unreachable. OAuth URIs are preferred
// when configured (matching the normal priority order).
func buildFallbackURIs(configs []ServerSecretConfig) map[string]string {
	secretNameToURI := make(map[string]string)

	for _, cfg := range configs {
		secretToOAuthID := oauthMapping(cfg)

		for _, s := range cfg.Secrets {
			if err := secret.ValidateSecretName(s.Name); err != nil {
				log.Logf("Warning: skipping secret with invalid name %q: %v", s.Name, err)
				continue
			}
			secretName := cfg.Namespace + s.Name
			if oauthSecretID, ok := secretToOAuthID[s.Name]; ok {
				secretNameToURI[secretName] = "se://" + oauthSecretID
			} else {
				secretNameToURI[secretName] = "se://" + secret.GetDefaultSecretKey(s.Name)
			}
		}
	}
	return secretNameToURI
}

// buildVerifiedURIs generates se:// URIs only for secrets that exist in the store.
// For servers with OAuth, checks OAuth token first, then falls back to direct secret.
func buildVerifiedURIs(configs []ServerSecretConfig, availableSecrets map[string]string) map[string]string {
	secretNameToURI := make(map[string]string)

	for _, cfg := range configs {
		secretToOAuthID := oauthMapping(cfg)

		for _, s := range cfg.Secrets {
			if err := secret.ValidateSecretName(s.Name); err != nil {
				log.Logf("Warning: skipping secret with invalid name %q: %v", s.Name, err)
				continue
			}
			secretName := cfg.Namespace + s.Name

			// Try OAuth token first
			if oauthSecretID, ok := secretToOAuthID[s.Name]; ok {
				if availableSecrets[oauthSecretID] != "" {
					secretNameToURI[secretName] = "se://" + oauthSecretID
					continue
				}
			}
			// Fall back to direct secret (API keys, tokens, etc.)
			secretID := secret.GetDefaultSecretKey(s.Name)
			if availableSecrets[secretID] != "" {
				secretNameToURI[secretName] = "se://" + secretID
			}
		}
	}
	return secretNameToURI
}

// oauthMapping builds a map from secret name to OAuth storage key for a server config.
func oauthMapping(cfg ServerSecretConfig) map[string]string {
	m := make(map[string]string)
	if cfg.OAuth != nil {
		for _, p := range cfg.OAuth.Providers {
			m[p.Secret] = secret.GetOAuthKey(p.Provider)
		}
	}
	return m
}
