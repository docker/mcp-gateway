package provider

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

// SwarmProvider implements SecretProvider using Docker Swarm Secrets
type SwarmProvider struct {
	client *client.Client
}

// NewSwarmProvider creates a new SwarmProvider
func NewSwarmProvider(dockerClient *client.Client) (*SwarmProvider, error) {
	if dockerClient == nil {
		return nil, fmt.Errorf("docker client cannot be nil")
	}
	return &SwarmProvider{client: dockerClient}, nil
}

// IsAvailable checks if Swarm is active
func (s *SwarmProvider) IsAvailable(ctx context.Context) bool {
	info, err := s.client.Info(ctx)
	if err != nil {
		return false
	}
	return info.Swarm.LocalNodeState == swarm.LocalNodeStateActive
}

// GetSecret returns the secret value (limitation: Swarm doesn't expose values for security)
func (s *SwarmProvider) GetSecret(ctx context.Context, name string) (string, error) {
	// Note: Docker Swarm doesn't expose secret values by security design
	// This method only confirms that the secret exists
	secrets, err := s.client.SecretList(ctx, types.SecretListOptions{
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil {
		return "", fmt.Errorf("listing secrets: %w", err)
	}

	if len(secrets) == 0 {
		return "", &SecretNotFoundError{Name: name, Provider: "swarm"}
	}

	// Swarm doesn't return values - only confirms existence
	// The actual value will be mounted by Docker Engine in the container
	return "", nil
}

// SetSecret creates a new Docker Swarm Secret
func (s *SwarmProvider) SetSecret(ctx context.Context, name, value string) error {
	// Check if it already exists
	existing, err := s.client.SecretList(ctx, types.SecretListOptions{
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil {
		return fmt.Errorf("checking existing secret: %w", err)
	}

	// If it already exists, we need to remove and recreate (secrets are immutable in Swarm)
	if len(existing) > 0 {
		// Note: In production, you may want to version secrets instead of deleting
		if err := s.client.SecretRemove(ctx, existing[0].ID); err != nil {
			return fmt.Errorf("removing existing secret: %w", err)
		}
	}

	secretSpec := swarm.SecretSpec{
		Annotations: swarm.Annotations{
			Name: name,
			Labels: map[string]string{
				"com.docker.mcp.managed": "true",
				"com.docker.mcp.version": "1",
			},
		},
		Data: []byte(value),
	}

	_, err = s.client.SecretCreate(ctx, secretSpec)
	if err != nil {
		return fmt.Errorf("creating swarm secret: %w", err)
	}

	return nil
}

// DeleteSecret removes a Swarm Secret
func (s *SwarmProvider) DeleteSecret(ctx context.Context, name string) error {
	secrets, err := s.client.SecretList(ctx, types.SecretListOptions{
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil {
		return fmt.Errorf("listing secrets: %w", err)
	}

	if len(secrets) == 0 {
		return &SecretNotFoundError{Name: name, Provider: "swarm"}
	}

	return s.client.SecretRemove(ctx, secrets[0].ID)
}

// ListSecrets lists all Swarm Secrets managed by MCP
func (s *SwarmProvider) ListSecrets(ctx context.Context) ([]StoredSecret, error) {
	secrets, err := s.client.SecretList(ctx, types.SecretListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "com.docker.mcp.managed=true")),
	})
	if err != nil {
		return nil, fmt.Errorf("listing swarm secrets: %w", err)
	}

	var result []StoredSecret
	for _, secret := range secrets {
		result = append(result, StoredSecret{
			Name:     secret.Spec.Name,
			Provider: "swarm",
		})
	}

	return result, nil
}

// SupportsSecureMount returns true - Swarm supports secure mounting
func (s *SwarmProvider) SupportsSecureMount() bool {
	return true
}

// ProviderName returns the provider name
func (s *SwarmProvider) ProviderName() string {
	return "swarm"
}

// GetSecretID returns the secret ID for use in mount strategies
func (s *SwarmProvider) GetSecretID(ctx context.Context, name string) (string, error) {
	secrets, err := s.client.SecretList(ctx, types.SecretListOptions{
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil {
		return "", fmt.Errorf("listing secrets: %w", err)
	}

	if len(secrets) == 0 {
		return "", &SecretNotFoundError{Name: name, Provider: "swarm"}
	}

	return secrets[0].ID, nil
}
