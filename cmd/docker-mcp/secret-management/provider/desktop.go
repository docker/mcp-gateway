package provider

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// DesktopProvider implements SecretProvider using Docker Desktop's secrets API.
type DesktopProvider struct {
	client *desktop.Secrets
}

// NewDesktopProvider creates a new DesktopProvider.
func NewDesktopProvider() *DesktopProvider {
	return &DesktopProvider{
		client: desktop.NewSecretsClient(),
	}
}

// GetSecret retrieves a secret from Docker Desktop.
func (d *DesktopProvider) GetSecret(ctx context.Context, name string) (string, error) {
	// Docker Desktop doesn't have a direct GetSecret method, so we list all secrets
	// and find the one we want. This is not ideal but matches existing behavior.
	secrets, err := d.client.ListJfsSecrets(ctx)
	if err != nil {
		return "", err
	}
	
	for _, secret := range secrets {
		if secret.Name == name {
			// Note: The Desktop API doesn't return secret values in list operations
			// This is a limitation of the current Desktop implementation
			return "", nil // Return empty value as per existing behavior
		}
	}
	
	return "", &SecretNotFoundError{Name: name, Provider: "docker-desktop"}
}

// SetSecret stores a secret in Docker Desktop.
func (d *DesktopProvider) SetSecret(ctx context.Context, name, value string) error {
	secret := desktop.Secret{
		Name:  name,
		Value: value,
	}
	
	return d.client.SetJfsSecret(ctx, secret)
}

// DeleteSecret removes a secret from Docker Desktop.
func (d *DesktopProvider) DeleteSecret(ctx context.Context, name string) error {
	return d.client.DeleteJfsSecret(ctx, name)
}

// ListSecrets returns all secrets from Docker Desktop.
func (d *DesktopProvider) ListSecrets(ctx context.Context) ([]StoredSecret, error) {
	desktopSecrets, err := d.client.ListJfsSecrets(ctx)
	if err != nil {
		return nil, err
	}
	
	var secrets []StoredSecret
	for _, secret := range desktopSecrets {
		secrets = append(secrets, StoredSecret{
			Name:     secret.Name,
			Provider: secret.Provider,
		})
	}
	
	return secrets, nil
}

// IsAvailable checks if Docker Desktop is available.
func (d *DesktopProvider) IsAvailable(ctx context.Context) bool {
	// Test if we can connect to Docker Desktop by trying to list secrets
	_, err := d.client.ListJfsSecrets(ctx)
	return err == nil
}

// ProviderName returns the name of this provider.
func (d *DesktopProvider) ProviderName() string {
	return "docker-desktop"
}

// SupportsSecureMount retorna true - Desktop suporta montagem segura via labels
func (d *DesktopProvider) SupportsSecureMount() bool {
	return true
}