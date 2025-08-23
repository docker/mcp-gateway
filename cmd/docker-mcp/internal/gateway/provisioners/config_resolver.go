package provisioners

import (
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/eval"
)

// GatewayConfigResolver implements ConfigResolver using a map of server configurations.
// This resolver provides just-in-time access to secrets and environment variables
// for the Docker virtual remote provisioner, maintaining separation from the client pool.
type GatewayConfigResolver struct {
	serverConfigs map[string]*catalog.ServerConfig
}

// NewGatewayConfigResolver creates a new config resolver with the given server configurations
func NewGatewayConfigResolver(serverConfigs map[string]*catalog.ServerConfig) *GatewayConfigResolver {
	return &GatewayConfigResolver{
		serverConfigs: serverConfigs,
	}
}

// ResolveSecrets returns the resolved secret values for the given server
func (gcr *GatewayConfigResolver) ResolveSecrets(serverName string) map[string]string {
	serverConfig, exists := gcr.serverConfigs[serverName]
	if !exists {
		return map[string]string{}
	}

	secretsMap := make(map[string]string)

	for _, secret := range serverConfig.Spec.Secrets {
		if secretValue, exists := serverConfig.Secrets[secret.Name]; exists {
			secretsMap[secret.Env] = secretValue
		} else {
			secretsMap[secret.Env] = "<UNKNOWN>"
		}
	}

	return secretsMap
}

// ResolveEnvironment returns the resolved environment variables for the given server
func (gcr *GatewayConfigResolver) ResolveEnvironment(serverName string) map[string]string {
	serverConfig, exists := gcr.serverConfigs[serverName]
	if !exists {
		return map[string]string{}
	}

	envMap := make(map[string]string)

	for _, envVar := range serverConfig.Spec.Env {
		var value string
		if strings.Contains(envVar.Value, "{{") && strings.Contains(envVar.Value, "}}") {
			// Apply template evaluation
			evalResult := eval.Evaluate(envVar.Value, serverConfig.Config)
			value = fmt.Sprintf("%v", evalResult)
		} else {
			value = envVar.Value
		}

		if value != "" {
			envMap[envVar.Name] = value
		}
	}

	return envMap
}

// ResolveCommand returns the resolved command with template substitution
func (gcr *GatewayConfigResolver) ResolveCommand(serverName string) []string {
	serverConfig, exists := gcr.serverConfigs[serverName]
	if !exists {
		return []string{}
	}

	// Apply template evaluation to command arguments
	return eval.EvaluateList(serverConfig.Spec.Command, serverConfig.Config)
}
