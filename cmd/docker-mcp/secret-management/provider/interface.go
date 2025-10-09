package provider

import (
	"context"
	"fmt"
)

// SecretProvider defines the interface for secret storage backends.
type SecretProvider interface {
	// GetSecret retrieves a secret by name.
	GetSecret(ctx context.Context, name string) (string, error)
	
	// SetSecret stores a secret with the given name and value.
	SetSecret(ctx context.Context, name, value string) error
	
	// DeleteSecret removes a secret by name.
	DeleteSecret(ctx context.Context, name string) error
	
	// ListSecrets returns all stored secrets.
	ListSecrets(ctx context.Context) ([]StoredSecret, error)
	
	// IsAvailable checks if this provider is currently available.
	IsAvailable(ctx context.Context) bool
	
	// ProviderName returns a human-readable name for this provider.
	ProviderName() string

	// SupportsSecureMount indicates whether the provider can mount without exposing values
	SupportsSecureMount() bool
}

// StoredSecret represents a secret stored in a provider.
type StoredSecret struct {
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
}

// ChainProvider provides fallback functionality by trying multiple providers in order.
type ChainProvider struct {
	providers []SecretProvider
}

// NewChainProvider creates a new ChainProvider with the given providers.
// Providers are tried in the order they are provided.
func NewChainProvider(providers ...SecretProvider) *ChainProvider {
	return &ChainProvider{
		providers: providers,
	}
}

// GetSecret tries to get a secret from each provider until one succeeds.
func (c *ChainProvider) GetSecret(ctx context.Context, name string) (string, error) {
	var lastErr error
	
	for _, provider := range c.providers {
		if !provider.IsAvailable(ctx) {
			continue
		}
		
		value, err := provider.GetSecret(ctx, name)
		if err == nil {
			return value, nil
		}
		lastErr = err
	}
	
	if lastErr != nil {
		return "", fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	
	return "", fmt.Errorf("no available providers")
}

// SetSecret stores a secret in the first available provider.
func (c *ChainProvider) SetSecret(ctx context.Context, name, value string) error {
	for _, provider := range c.providers {
		if !provider.IsAvailable(ctx) {
			continue
		}
		
		return provider.SetSecret(ctx, name, value)
	}
	
	return fmt.Errorf("no available providers for storing secret")
}

// DeleteSecret removes a secret from all providers.
func (c *ChainProvider) DeleteSecret(ctx context.Context, name string) error {
	var errors []error
	deleted := false
	
	for _, provider := range c.providers {
		if !provider.IsAvailable(ctx) {
			continue
		}
		
		err := provider.DeleteSecret(ctx, name)
		if err == nil {
			deleted = true
		} else {
			errors = append(errors, fmt.Errorf("%s: %w", provider.ProviderName(), err))
		}
	}
	
	if deleted {
		return nil // At least one provider succeeded
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to delete from all providers: %v", errors)
	}
	
	return fmt.Errorf("no available providers")
}

// ListSecrets returns secrets from all available providers.
func (c *ChainProvider) ListSecrets(ctx context.Context) ([]StoredSecret, error) {
	var allSecrets []StoredSecret
	seen := make(map[string]bool)
	
	for _, provider := range c.providers {
		if !provider.IsAvailable(ctx) {
			continue
		}
		
		secrets, err := provider.ListSecrets(ctx)
		if err != nil {
			continue // Skip providers that fail to list
		}
		
		for _, secret := range secrets {
			if !seen[secret.Name] {
				secret.Provider = provider.ProviderName()
				allSecrets = append(allSecrets, secret)
				seen[secret.Name] = true
			}
		}
	}
	
	return allSecrets, nil
}

// IsAvailable returns true if at least one provider is available.
func (c *ChainProvider) IsAvailable(ctx context.Context) bool {
	for _, provider := range c.providers {
		if provider.IsAvailable(ctx) {
			return true
		}
	}
	return false
}

// ProviderName returns the name of the chain provider.
func (c *ChainProvider) ProviderName() string {
	return "chain"
}

// SupportsSecureMount returns true if at least one provider supports secure mount
func (c *ChainProvider) SupportsSecureMount() bool {
	for _, provider := range c.providers {
		if provider.SupportsSecureMount() {
			return true
		}
	}
	return false
}

// GetDefaultProvider returns the default secret provider chain.
// It tries Docker Desktop first, then credential store, then file storage.
func GetDefaultProvider() SecretProvider {
	return NewChainProvider(
		NewDesktopProvider(),
		NewCredStoreProvider(),
		NewFileProvider(),
	)
}