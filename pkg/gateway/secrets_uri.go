package gateway

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/catalog"
)

// ServerSecretsInput represents the information needed to build secrets URIs for a server
type ServerSecretsInput struct {
	Secrets        []catalog.Secret // Secret definitions from server catalog
	OAuth          *catalog.OAuth   // OAuth config (nil if no OAuth)
	ProviderPrefix string           // Optional prefix for map keys (e.g., "default_" for WorkingSet namespacing)
}

// BuildSecretsURIsOptions configures behavior of BuildSecretsURIs
type BuildSecretsURIsOptions struct {
	// RequireSecretExists: When true, queries Secrets Engine to verify secrets exist
	// before building URIs. When false, builds URIs for all secrets without verification.
	//
	// Use true for FileBasedConfiguration (legacy mode) - matches original behavior
	// Use false for WorkingSetConfiguration (profile mode) - Docker Desktop validates separately
	RequireSecretExists bool
}

// BuildSecretsURIs generates se:// URIs for the given server inputs.
//
// Behavior depends on opts.RequireSecretExists:
//   - false: Build URIs for all secrets without querying Secrets Engine (WorkingSetConfiguration)
//   - true: Query Secrets Engine, only build URIs for existing secrets, OAuth priority (FileBasedConfiguration)
func BuildSecretsURIs(ctx context.Context, inputs []ServerSecretsInput, opts BuildSecretsURIsOptions) map[string]string {
	uris := make(map[string]string)

	// WorkingSetConfiguration: don't call GetSecrets, just build URIs directly
	// This preserves the original behavior where no Secrets Engine call was made during config reading
	if !opts.RequireSecretExists {
		for _, input := range inputs {
			for _, s := range input.Secrets {
				key := secret.GetDefaultSecretKey(s.Name)
				mapKey := input.ProviderPrefix + s.Name
				uris[mapKey] = fmt.Sprintf("se://%s", key)
			}
		}
		return uris
	}

	// FileBasedConfiguration: call GetSecrets to verify secrets exist and handle OAuth
	allSecrets, _ := secret.GetSecrets(ctx)
	secretsMap := make(map[string]string)
	for _, e := range allSecrets {
		secretsMap[e.ID] = string(e.Value)
	}

	for _, input := range inputs {
		// Server has no OAuth configured - use secret directly
		if input.OAuth == nil {
			for _, s := range input.Secrets {
				key := secret.GetDefaultSecretKey(s.Name)
				if secretsMap[key] != "" {
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
				if secretsMap[oauthKey] != "" {
					uris[s.Name] = "se://" + oauthKey
					continue
				}
			}
			// Fallback to PAT (must be non-empty)
			patKey := secret.GetDefaultSecretKey(s.Name)
			if secretsMap[patKey] != "" {
				uris[s.Name] = "se://" + patKey
			}
		}
	}
	return uris
}
