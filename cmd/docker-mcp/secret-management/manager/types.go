package manager

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/provider"
)

// SecretMode define como os secrets são tratados
type SecretMode string

const (
	// ReferenceModeOnly - Gateway nunca conhece valores (produção)
	ReferenceModeOnly SecretMode = "reference"

	// ValueMode - Gateway pode ler valores (desenvolvimento/migração)
	ValueMode SecretMode = "value"

	// HybridMode - Tenta reference, fallback para value
	HybridMode SecretMode = "hybrid"
)

// SecretReference representa uma referência a um secret (não o valor)
type SecretReference struct {
	Name          string
	Provider      string
	MountStrategy MountStrategy
}

// MountStrategy define como montar o secret no container
type MountStrategy interface {
	// Apply configura o container para receber o secret
	Apply(containerConfig *container.Config, hostConfig *container.HostConfig) error

	// Type retorna o tipo de estratégia
	Type() string
}

// EnvironmentCapabilities descreve o que o ambiente suporta
type EnvironmentCapabilities struct {
	HasDockerDesktop     bool
	HasSwarmMode         bool
	HasCredentialHelper  bool
	SupportsSecureMount  bool
	RecommendedStrategy  string
}

// SecretManager orquestra o gerenciamento de secrets
type SecretManager interface {
	// GetSecretReference obtém referência para montar secret
	GetSecretReference(ctx context.Context, name string) (*SecretReference, error)

	// GetSecretValue obtém valor real (apenas em ValueMode)
	GetSecretValue(ctx context.Context, name string) (string, error)

	// SetSecret armazena um secret
	SetSecret(ctx context.Context, name, value string) error

	// DeleteSecret remove um secret
	DeleteSecret(ctx context.Context, name string) error

	// ListSecrets lista secrets disponíveis
	ListSecrets(ctx context.Context) ([]provider.StoredSecret, error)

	// GetMode retorna o modo de operação atual
	GetMode() SecretMode

	// DetectEnvironment identifica capacidades do ambiente
	DetectEnvironment(ctx context.Context) (*EnvironmentCapabilities, error)
}
