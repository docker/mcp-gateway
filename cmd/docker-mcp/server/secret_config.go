package server

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// getConfiguredSecretNames returns a map of configured secret names for quick lookup.
// This is a shared helper used by both ls.go and enable.go.
func getConfiguredSecretNames(ctx context.Context) (map[string]struct{}, error) {
	secretsClient := desktop.NewSecretsClient()
	configuredSecrets, err := secretsClient.ListJfsSecrets(ctx)
	if err != nil {
		return nil, err
	}

	configuredSecretNames := make(map[string]struct{})
	for _, secret := range configuredSecrets {
		configuredSecretNames[secret.Name] = struct{}{}
	}

	return configuredSecretNames, nil
}
