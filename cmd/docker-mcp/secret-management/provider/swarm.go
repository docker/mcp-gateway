package provider

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

// SwarmProvider implementa SecretProvider usando Docker Swarm Secrets
type SwarmProvider struct {
	client *client.Client
}

// NewSwarmProvider cria um novo SwarmProvider
func NewSwarmProvider(dockerClient *client.Client) (*SwarmProvider, error) {
	if dockerClient == nil {
		return nil, fmt.Errorf("docker client cannot be nil")
	}
	return &SwarmProvider{client: dockerClient}, nil
}

// IsAvailable verifica se Swarm está ativo
func (s *SwarmProvider) IsAvailable(ctx context.Context) bool {
	info, err := s.client.Info(ctx)
	if err != nil {
		return false
	}
	return info.Swarm.LocalNodeState == swarm.LocalNodeStateActive
}

// GetSecret retorna valor do secret (limitação: Swarm não expõe valores por segurança)
func (s *SwarmProvider) GetSecret(ctx context.Context, name string) (string, error) {
	// Nota: Docker Swarm não expõe valores de secrets por design de segurança
	// Este método apenas confirma que o secret existe
	secrets, err := s.client.SecretList(ctx, types.SecretListOptions{
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil {
		return "", fmt.Errorf("listing secrets: %w", err)
	}

	if len(secrets) == 0 {
		return "", &SecretNotFoundError{Name: name, Provider: "swarm"}
	}

	// Swarm não retorna valores - apenas confirma existência
	// O valor real será montado pelo Docker Engine no container
	return "", nil
}

// SetSecret cria um novo Docker Swarm Secret
func (s *SwarmProvider) SetSecret(ctx context.Context, name, value string) error {
	// Verifica se já existe
	existing, err := s.client.SecretList(ctx, types.SecretListOptions{
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil {
		return fmt.Errorf("checking existing secret: %w", err)
	}

	// Se já existe, precisamos remover e recriar (secrets são imutáveis no Swarm)
	if len(existing) > 0 {
		// Nota: Em produção, você pode querer versionar secrets ao invés de deletar
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

// DeleteSecret remove um Swarm Secret
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

// ListSecrets lista todos os Swarm Secrets gerenciados pelo MCP
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

// SupportsSecureMount retorna true - Swarm suporta montagem segura
func (s *SwarmProvider) SupportsSecureMount() bool {
	return true
}

// ProviderName retorna o nome do provider
func (s *SwarmProvider) ProviderName() string {
	return "swarm"
}

// GetSecretID retorna o ID do secret para uso em mount strategies
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
