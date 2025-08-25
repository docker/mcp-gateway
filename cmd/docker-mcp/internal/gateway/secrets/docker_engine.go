package secrets

import (
	"context"
	"strings"
)

// ConfigResolver interface to avoid circular import
// This mirrors provisioners.ConfigResolver interface
type ConfigResolver interface {
	ResolveSecrets(serverName string) map[string]string
	ResolveEnvironment(serverName string) map[string]string
	ResolveCommand(serverName string) []string
}

// DockerEngineSecretProvider resolves templates using Docker Desktop credential store
type DockerEngineSecretProvider struct {
	configResolver ConfigResolver
}

// NewDockerEngineSecretProvider creates a new Docker Engine secret provider
func NewDockerEngineSecretProvider(configResolver any) *DockerEngineSecretProvider {
	// Type assert to our ConfigResolver interface
	cr, ok := configResolver.(ConfigResolver)
	if !ok {
		// This shouldn't happen if used correctly, but handle gracefully
		panic("configResolver does not implement required interface")
	}

	return &DockerEngineSecretProvider{
		configResolver: cr,
	}
}

// GetName returns the provider name
func (d *DockerEngineSecretProvider) GetName() string {
	return "docker-engine"
}

// ResolveSecrets resolves templates using Docker Desktop credential store
func (d *DockerEngineSecretProvider) ResolveSecrets(_ context.Context, templates map[string]string) (map[string]string, error) {
	resolved := make(map[string]string)

	// Group templates by server name for efficient resolution
	// Since ConfigResolver.ResolveSecrets() expects a serverName, we need to extract
	// the server context or use a dummy server name for template resolution

	// For now, we'll use a generic approach and resolve templates one by one
	// TODO: This may need optimization if we have server-specific template resolution

	for envVar, template := range templates {
		// Extract template content
		templateKey := extractTemplateKey(template)

		// Use the configResolver to get resolved secrets
		// Note: This assumes the configResolver can handle individual template resolution
		// The actual server name may be needed for proper resolution
		secrets := d.configResolver.ResolveSecrets("") // Empty server name for generic resolution

		if value, exists := secrets[templateKey]; exists {
			resolved[envVar] = value
		} else {
			// Template not found, leave as template for debugging
			resolved[envVar] = template
		}
	}

	return resolved, nil
}

// GetSecretStrategy returns the injection strategy for Docker containers
func (d *DockerEngineSecretProvider) GetSecretStrategy() SecretStrategy {
	return SecretStrategyEnvVars
}

// extractTemplateKey removes {{ }} wrapper from templates
func extractTemplateKey(template string) string {
	key := strings.TrimPrefix(template, "{{")
	key = strings.TrimSuffix(key, "}}")
	return key
}
