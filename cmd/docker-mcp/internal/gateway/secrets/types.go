package secrets

import (
	"context"
	"fmt"
)

// SecretProvider defines how to resolve secret templates into actual values
type SecretProvider interface {
	// GetName returns the unique identifier for this secret provider type
	GetName() string

	// ResolveSecrets resolves template strings to actual secret values
	// Input: map of template strings (e.g., "{{dockerhub.username}}")
	// Output: map of resolved values
	ResolveSecrets(ctx context.Context, templates map[string]string) (map[string]string, error)

	// GetSecretStrategy returns how this provider expects secrets to be used
	GetSecretStrategy() SecretStrategy
}

// SecretStrategy defines how resolved secrets should be injected into containers
type SecretStrategy string

const (
	// SecretStrategyEnvVars - inject directly as environment variables (Docker)
	SecretStrategyEnvVars SecretStrategy = "env-vars"

	// SecretStrategySecretKeyRef - create Secret resources and use secretKeyRef (K8s development)
	SecretStrategySecretKeyRef SecretStrategy = "secretKeyRef"

	// SecretStrategyReference - reference pre-existing Secret resources (K8s production)
	SecretStrategyReference SecretStrategy = "reference"

	// SecretStrategyExternal - external secret managers (future)
	SecretStrategyExternal SecretStrategy = "external"
)

// SecretProviderType represents supported secret provider types
type SecretProviderType string

const (
	// DockerEngine uses Docker Desktop credential store
	DockerEngine SecretProviderType = "docker-engine"

	// Cluster assumes secrets pre-exist in Kubernetes cluster
	Cluster SecretProviderType = "cluster"
)

// ParseSecretProviderType converts string to SecretProviderType
func ParseSecretProviderType(s string) (SecretProviderType, error) {
	switch s {
	case "docker-engine":
		return DockerEngine, nil
	case "cluster":
		return Cluster, nil
	default:
		return DockerEngine, fmt.Errorf("unknown secret provider type: %s", s)
	}
}

// SecretProviderConfig holds configuration for creating secret providers
type SecretProviderConfig struct {
	Type       SecretProviderType
	SecretName string // For cluster provider
	Namespace  string // For cluster provider

	// Dependencies
	ConfigResolver any // provisioners.ConfigResolver for docker-engine provider
}

// NewSecretProvider creates a secret provider based on configuration
func NewSecretProvider(config SecretProviderConfig) (SecretProvider, error) {
	switch config.Type {
	case DockerEngine:
		if config.ConfigResolver == nil {
			return nil, fmt.Errorf("ConfigResolver required for docker-engine secret provider")
		}
		// Type assertion to avoid circular import
		// The actual implementation will need to import the right interface
		return NewDockerEngineSecretProvider(config.ConfigResolver), nil

	case Cluster:
		return NewClusterSecretProvider(config.SecretName, config.Namespace), nil

	default:
		return nil, fmt.Errorf("unsupported secret provider type: %s", config.Type)
	}
}
