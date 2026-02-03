package server

import (
	"context"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
)

// getConfiguredSecretNames returns a map of configured secret names for quick lookup.
// This is a shared helper used by both ls.go and enable.go.
func getConfiguredSecretNames(ctx context.Context) (map[string]struct{}, error) {
	envelopes, err := secret.GetSecrets(ctx)
	if err != nil {
		return nil, err
	}

	configuredSecretNames := make(map[string]struct{})
	for _, env := range envelopes {
		// Extract base name from full ID using centralized namespace stripper
		name := secret.StripNamespace(env.ID)
		configuredSecretNames[name] = struct{}{}
	}

	return configuredSecretNames, nil
}
