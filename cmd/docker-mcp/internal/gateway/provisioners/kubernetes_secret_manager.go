package provisioners

import (
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/secrets"
)

// GatewayKubernetesSecretManager implements KubernetesSecretManager using gateway server configurations
type GatewayKubernetesSecretManager struct {
	configResolver ConfigResolver // Compose with ConfigResolver for template resolution
	serverConfigs  map[string]*catalog.ServerConfig
	secretName     string // Gateway-level secret name (e.g., "mcp-gateway-secrets")
}

// NewGatewayKubernetesSecretManager creates a new Kubernetes secret manager
func NewGatewayKubernetesSecretManager(configResolver ConfigResolver, serverConfigs map[string]*catalog.ServerConfig, secretName string) *GatewayKubernetesSecretManager {
	return &GatewayKubernetesSecretManager{
		configResolver: configResolver,
		serverConfigs:  serverConfigs,
		secretName:     secretName,
	}
}

// GetSecretSpecs returns Kubernetes Secret resource specifications
func (gksm *GatewayKubernetesSecretManager) GetSecretSpecs(serverName string) map[string]map[string]string {
	// If serverName is empty, collect secrets from all servers
	if serverName == "" {
		return gksm.getAllSecrets()
	}

	serverConfig, exists := gksm.serverConfigs[serverName]
	if !exists {
		return map[string]map[string]string{}
	}

	// Use the gateway-level secret name (configurable, e.g., "mcp-gateway-secrets")
	secretData := make(map[string]string)

	// Use ConfigResolver to get resolved secret values
	resolvedSecrets := gksm.configResolver.ResolveSecrets(serverName)

	// Map resolved secrets to Kubernetes Secret data structure
	for _, secret := range serverConfig.Spec.Secrets {
		if secretValue, exists := resolvedSecrets[secret.Env]; exists {
			// Use consistent template mapping: secret.Name is template, convert to secret key
			secretKey := secrets.TemplateToSecretKey(secret.Name)
			secretData[secretKey] = secretValue
		}
	}

	// Only return the secret if it has data
	if len(secretData) > 0 {
		return map[string]map[string]string{
			gksm.secretName: secretData,
		}
	}

	return map[string]map[string]string{}
}

// getAllSecrets returns all secrets from all servers as a single shared secret
func (gksm *GatewayKubernetesSecretManager) getAllSecrets() map[string]map[string]string {
	allSecretData := make(map[string]string)

	// Iterate through all server configs and collect their secrets
	for serverName, serverConfig := range gksm.serverConfigs {
		if len(serverConfig.Spec.Secrets) == 0 {
			continue // Skip servers with no secrets
		}

		// Use ConfigResolver to get resolved secret values for this server
		resolvedSecrets := gksm.configResolver.ResolveSecrets(serverName)

		// Map resolved secrets to Kubernetes Secret data structure
		for _, secret := range serverConfig.Spec.Secrets {
			if secretValue, exists := resolvedSecrets[secret.Env]; exists {
				// Use consistent template mapping: secret.Name is template, convert to secret key
				secretKey := secrets.TemplateToSecretKey(secret.Name)
				allSecretData[secretKey] = secretValue
			}
		}
	}

	// Only return the shared secret if it has data
	if len(allSecretData) > 0 {
		return map[string]map[string]string{
			gksm.secretName: allSecretData,
		}
	}

	return map[string]map[string]string{}
}

// GetSecretKeyRefs returns environment variable to secretKeyRef mappings
func (gksm *GatewayKubernetesSecretManager) GetSecretKeyRefs(serverName string) map[string]SecretKeyRef {
	serverConfig, exists := gksm.serverConfigs[serverName]
	if !exists {
		return map[string]SecretKeyRef{}
	}

	secretKeyRefs := make(map[string]SecretKeyRef)

	// Map each secret's environment variable to a secretKeyRef
	for _, secret := range serverConfig.Spec.Secrets {
		if _, exists := serverConfig.Secrets[secret.Name]; exists {
			// Use consistent template mapping: secret.Name is the template like "{{dockerhub.username}}"
			secretKey := secrets.TemplateToSecretKey(secret.Name)

			secretKeyRefs[secret.Env] = SecretKeyRef{
				Name: gksm.secretName,
				Key:  secretKey,
			}
		}
	}

	return secretKeyRefs
}
