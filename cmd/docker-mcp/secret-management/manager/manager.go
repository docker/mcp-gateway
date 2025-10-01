package manager

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/docker/docker/client"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/provider"
)

type secretManager struct {
	mode      SecretMode
	providers []provider.SecretProvider
	detector  *EnvironmentDetector
	env       *EnvironmentCapabilities
}

// NewSecretManager creates a new secrets manager
func NewSecretManager(
	ctx context.Context,
	mode SecretMode,
	dockerClient *client.Client,
) (SecretManager, error) {
	detector := NewEnvironmentDetector(dockerClient)
	env, err := detector.Detect(ctx)
	if err != nil {
		return nil, fmt.Errorf("detecting environment: %w", err)
	}

	// Log environment detection
	logEnvironmentDetection(env)

	// Initialize providers in priority order
	providers := []provider.SecretProvider{}

	// 1. SwarmProvider (if available)
	if env.HasSwarmMode {
		swarmProvider, err := provider.NewSwarmProvider(dockerClient)
		if err == nil && swarmProvider.IsAvailable(ctx) {
			providers = append(providers, swarmProvider)
			log.Printf("[SecretManager] SwarmProvider added (priority: 1)")
		}
	}

	// 2. DesktopProvider (if available)
	if env.HasDockerDesktop {
		desktopProvider := provider.NewDesktopProvider()
		if desktopProvider.IsAvailable(ctx) {
			providers = append(providers, desktopProvider)
			log.Printf("[SecretManager] DesktopProvider added (priority: 2)")
		}
	}

	// 3. CredStoreProvider (if available)
	if env.HasCredentialHelper {
		credProvider := provider.NewCredStoreProvider()
		if credProvider.IsAvailable(ctx) {
			providers = append(providers, credProvider)
			log.Printf("[SecretManager] CredStoreProvider added (priority: 3)")
		}
	}

	// 4. FileProvider (always available as fallback)
	fileProvider := provider.NewFileProvider()
	providers = append(providers, fileProvider)
	log.Printf("[SecretManager] FileProvider added (fallback)")

	if len(providers) == 0 {
		return nil, fmt.Errorf("no secret providers available")
	}

	return &secretManager{
		mode:      mode,
		providers: providers,
		detector:  detector,
		env:       env,
	}, nil
}

func (m *secretManager) GetSecretReference(ctx context.Context, name string) (*SecretReference, error) {
	// Try each provider that supports secure mount
	for _, p := range m.providers {
		if !p.SupportsSecureMount() {
			continue
		}

		// Check if secret exists
		_, err := p.GetSecret(ctx, name)
		if err != nil {
			continue
		}

		// Create appropriate mount strategy
		strategy, err := m.getMountStrategy(ctx, p, name)
		if err != nil {
			continue
		}

		log.Printf("[SecretManager] Found secret '%s' in provider '%s' (secure mount)", name, p.ProviderName())

		return &SecretReference{
			Name:          name,
			Provider:      p.ProviderName(),
			MountStrategy: strategy,
		}, nil
	}

	// Fallback to value mode if configured
	if m.mode == HybridMode || m.mode == ValueMode {
		return m.getFallbackReference(ctx, name)
	}

	return nil, fmt.Errorf("secret %s not found in any provider supporting secure mount", name)
}

func (m *secretManager) getMountStrategy(ctx context.Context, p provider.SecretProvider, name string) (MountStrategy, error) {
	switch p.ProviderName() {
	case "swarm":
		// Para Swarm, precisamos do secret ID
		if swarmProvider, ok := p.(*provider.SwarmProvider); ok {
			secretID, err := swarmProvider.GetSecretID(ctx, name)
			if err != nil {
				return nil, err
			}
			return &SwarmSecretStrategy{
				SecretID:   secretID,
				SecretName: name,
				TargetPath: name,
				Mode:       0400,
			}, nil
		}

	case "docker-desktop":
		return &DesktopLabelStrategy{
			SecretName: name,
			MountPath:  fmt.Sprintf("/run/secrets/%s", name),
		}, nil
	}

	return nil, fmt.Errorf("unsupported provider for secure mount: %s", p.ProviderName())
}

func (m *secretManager) GetSecretValue(ctx context.Context, name string) (string, error) {
	// Only allowed in ValueMode
	if m.mode == ReferenceModeOnly {
		return "", fmt.Errorf("GetSecretValue not allowed in reference-only mode")
	}

	log.Printf("[SecretManager] Reading secret value '%s' (mode: %s)", name, m.mode)

	// Try each provider
	for _, p := range m.providers {
		if !p.IsAvailable(ctx) {
			continue
		}

		value, err := p.GetSecret(ctx, name)
		if err == nil && value != "" {
			log.Printf("[SecretManager] Secret '%s' found in provider '%s'", name, p.ProviderName())
			return value, nil
		}
	}

	return "", fmt.Errorf("secret %s not found in any provider", name)
}

func (m *secretManager) SetSecret(ctx context.Context, name, value string) error {
	// Use the first available provider
	if len(m.providers) == 0 {
		return fmt.Errorf("no secret providers available")
	}

	for _, p := range m.providers {
		if !p.IsAvailable(ctx) {
			continue
		}

		log.Printf("[SecretManager] Storing secret '%s' in provider '%s'", name, p.ProviderName())
		return p.SetSecret(ctx, name, value)
	}

	return fmt.Errorf("no available providers for storing secret")
}

func (m *secretManager) DeleteSecret(ctx context.Context, name string) error {
	// Try to delete from all providers
	deleted := false
	var lastErr error

	for _, p := range m.providers {
		if !p.IsAvailable(ctx) {
			continue
		}

		err := p.DeleteSecret(ctx, name)
		if err == nil {
			deleted = true
			log.Printf("[SecretManager] Secret '%s' deleted from provider '%s'", name, p.ProviderName())
		} else {
			lastErr = err
		}
	}

	if !deleted && lastErr != nil {
		return lastErr
	}

	return nil
}

func (m *secretManager) ListSecrets(ctx context.Context) ([]provider.StoredSecret, error) {
	// Aggregate secrets from all providers
	allSecrets := []provider.StoredSecret{}
	seen := make(map[string]bool)

	for _, p := range m.providers {
		if !p.IsAvailable(ctx) {
			continue
		}

		secrets, err := p.ListSecrets(ctx)
		if err != nil {
			log.Printf("[SecretManager] Failed to list from provider '%s': %v", p.ProviderName(), err)
			continue
		}

		for _, secret := range secrets {
			if !seen[secret.Name] {
				allSecrets = append(allSecrets, secret)
				seen[secret.Name] = true
			}
		}
	}

	return allSecrets, nil
}

func (m *secretManager) GetMode() SecretMode {
	return m.mode
}

func (m *secretManager) DetectEnvironment(ctx context.Context) (*EnvironmentCapabilities, error) {
	return m.env, nil
}

func (m *secretManager) getFallbackReference(ctx context.Context, name string) (*SecretReference, error) {
	// For providers that don't support secure mount,
	// create a tmpfs strategy
	value, err := m.GetSecretValue(ctx, name)
	if err != nil {
		return nil, err
	}

	log.Printf("[SecretManager] Using tmpfs fallback for secret '%s'", name)

	return &SecretReference{
		Name:     name,
		Provider: "fallback",
		MountStrategy: &TmpfsMountStrategy{
			SecretName:  name,
			SecretValue: value,
			MountPath:   fmt.Sprintf("/run/secrets/%s", name),
		},
	}, nil
}

// logEnvironmentDetection logs detected capabilities
func logEnvironmentDetection(env *EnvironmentCapabilities) {
	if os.Getenv("MCP_SECRET_DEBUG") != "" {
		log.Printf("[SecretManager] Environment detected:")
		log.Printf("  Docker Desktop: %v", env.HasDockerDesktop)
		log.Printf("  Swarm Mode: %v", env.HasSwarmMode)
		log.Printf("  Credential Helper: %v", env.HasCredentialHelper)
		log.Printf("  Secure Mount: %v", env.SupportsSecureMount)
		log.Printf("  Recommended: %s", env.RecommendedStrategy)
	}
}
