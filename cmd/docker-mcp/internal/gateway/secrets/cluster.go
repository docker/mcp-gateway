package secrets

import (
	"context"
)

// ClusterSecretProvider assumes secrets pre-exist in Kubernetes cluster
type ClusterSecretProvider struct {
	secretName string // Name of the Secret resource (e.g., "mcp-gateway-secrets")
	namespace  string // Kubernetes namespace
}

// NewClusterSecretProvider creates a new cluster secret provider
func NewClusterSecretProvider(secretName, namespace string) *ClusterSecretProvider {
	return &ClusterSecretProvider{
		secretName: secretName,
		namespace:  namespace,
	}
}

// GetName returns the provider name
func (c *ClusterSecretProvider) GetName() string {
	return "cluster"
}

// ResolveSecrets leaves templates unresolved, assuming they'll be referenced via secretKeyRef
func (c *ClusterSecretProvider) ResolveSecrets(_ context.Context, templates map[string]string) (map[string]string, error) {
	resolved := make(map[string]string)

	// In cluster mode, we don't actually resolve templates to values
	// Instead, we map them to the expected secret key format for Kubernetes secretKeyRef
	for envVar, template := range templates {
		secretKey := TemplateToSecretKey(template)

		// Store the secret key reference information
		// The actual Pod spec will use secretKeyRef: {name: secretName, key: secretKey}
		resolved[envVar] = secretKey
	}

	return resolved, nil
}

// GetSecretStrategy returns the injection strategy for Kubernetes
func (c *ClusterSecretProvider) GetSecretStrategy() SecretStrategy {
	return SecretStrategyReference
}

// GetSecretName returns the configured secret name
func (c *ClusterSecretProvider) GetSecretName() string {
	return c.secretName
}

// GetNamespace returns the configured namespace
func (c *ClusterSecretProvider) GetNamespace() string {
	return c.namespace
}
