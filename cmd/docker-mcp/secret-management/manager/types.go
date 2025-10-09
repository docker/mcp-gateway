package manager

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/provider"
)

// SecretMode defines how secrets are handled
type SecretMode string

const (
	// ReferenceModeOnly - Gateway never knows values (production)
	ReferenceModeOnly SecretMode = "reference"

	// ValueMode - Gateway can read values (development/migration)
	ValueMode SecretMode = "value"

	// HybridMode - Tries reference, falls back to value
	HybridMode SecretMode = "hybrid"
)

// SecretReference represents a reference to a secret (not the value)
type SecretReference struct {
	Name          string
	Provider      string
	MountStrategy MountStrategy
}

// MountStrategy defines how to mount the secret in the container
type MountStrategy interface {
	// Apply configures the container to receive the secret
	Apply(containerConfig *container.Config, hostConfig *container.HostConfig) error

	// Type returns the strategy type
	Type() string
}

// EnvironmentCapabilities describes what the environment supports
type EnvironmentCapabilities struct {
	HasDockerDesktop     bool
	HasSwarmMode         bool
	HasCredentialHelper  bool
	SupportsSecureMount  bool
	RecommendedStrategy  string
}

// SecretManager orchestrates secret management
type SecretManager interface {
	// GetSecretReference gets a reference to mount the secret
	GetSecretReference(ctx context.Context, name string) (*SecretReference, error)

	// GetSecretValue gets the actual value (only in ValueMode)
	GetSecretValue(ctx context.Context, name string) (string, error)

	// SetSecret stores a secret
	SetSecret(ctx context.Context, name, value string) error

	// DeleteSecret removes a secret
	DeleteSecret(ctx context.Context, name string) error

	// ListSecrets lists available secrets
	ListSecrets(ctx context.Context) ([]provider.StoredSecret, error)

	// GetMode returns the current operation mode
	GetMode() SecretMode

	// DetectEnvironment identifies environment capabilities
	DetectEnvironment(ctx context.Context) (*EnvironmentCapabilities, error)
}
