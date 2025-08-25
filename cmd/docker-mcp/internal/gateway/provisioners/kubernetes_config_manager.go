package provisioners

import (
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

// GatewayKubernetesConfigManager implements KubernetesConfigManager using gateway server configurations
// This manager is ONLY used for DockerEngineConfigProvider mode where ConfigMaps are created dynamically.
// In ClusterConfigProvider mode, pre-existing ConfigMaps are used directly via configName references.
type GatewayKubernetesConfigManager struct {
	configResolver ConfigResolver // Compose with ConfigResolver for template resolution
	serverConfigs  map[string]*catalog.ServerConfig
	configName     string // Gateway-level ConfigMap name (e.g., "mcp-gateway-config")
	isDockerEngine bool   // true = docker-engine mode (resolve templates), false = cluster mode (skip templates)
}

// NewGatewayKubernetesConfigManager creates a new Kubernetes ConfigMap manager for DockerEngineConfigProvider mode.
// This manager resolves templates and creates ConfigMaps dynamically.
func NewGatewayKubernetesConfigManager(configResolver ConfigResolver, serverConfigs map[string]*catalog.ServerConfig, configName string) *GatewayKubernetesConfigManager {
	return &GatewayKubernetesConfigManager{
		configResolver: configResolver,
		serverConfigs:  serverConfigs,
		configName:     configName,
		isDockerEngine: true, // This constructor is only called for DockerEngineConfigProvider mode
	}
}

// NewGatewayKubernetesConfigManagerForCluster creates a config manager for ClusterConfigProvider mode.
// This should normally NOT be used since cluster mode uses pre-existing ConfigMaps,
// but this provides defensive behavior if somehow a ConfigManager is created in cluster mode.
func NewGatewayKubernetesConfigManagerForCluster(configResolver ConfigResolver, serverConfigs map[string]*catalog.ServerConfig, configName string) *GatewayKubernetesConfigManager {
	return &GatewayKubernetesConfigManager{
		configResolver: configResolver,
		serverConfigs:  serverConfigs,
		configName:     configName,
		isDockerEngine: false, // Cluster mode - skip templated environment variables
	}
}

// GetConfigSpecs returns Kubernetes ConfigMap resource specifications for DockerEngineConfigProvider mode.
// Templates are resolved here because this manager is only used when ConfigMaps are created dynamically.
// For ClusterConfigProvider mode, this manager is not used - pre-existing ConfigMaps are referenced directly.
func (gkcm *GatewayKubernetesConfigManager) GetConfigSpecs(serverName string) map[string]map[string]string {
	// If serverName is empty, collect configs from all servers
	if serverName == "" {
		return gkcm.getAllConfigs()
	}

	serverConfig, exists := gkcm.serverConfigs[serverName]
	if !exists {
		return map[string]map[string]string{}
	}

	// Use the gateway-level ConfigMap name (configurable, e.g., "mcp-gateway-config")
	configData := make(map[string]string)

	if gkcm.isDockerEngine {
		// Docker-engine mode: Resolve all templates and include all environment variables
		// Use ConfigResolver to get resolved environment values (non-secret)
		resolvedEnv := gkcm.configResolver.ResolveEnvironment(serverName)

		// Map resolved environment variables to Kubernetes ConfigMap data structure
		for _, env := range serverConfig.Spec.Env {
			if envValue, exists := resolvedEnv[env.Name]; exists {
				// Use the environment variable name as the ConfigMap key
				configData[env.Name] = envValue
			}
		}
	} else {
		// Cluster mode: Only include non-templated (static) environment variables
		// Templated variables are expected to be provided out-of-band in pre-existing ConfigMaps
		for _, env := range serverConfig.Spec.Env {
			if strings.Contains(env.Value, "{{") && strings.Contains(env.Value, "}}") {
				// Skip templated values - expect them in pre-existing ConfigMaps
				continue
			}
			// Only include static (non-templated) values in our ConfigMaps
			if env.Value != "" {
				configData[env.Name] = env.Value
			}
		}
	}

	// Only return the ConfigMap if it has data
	if len(configData) > 0 {
		return map[string]map[string]string{
			gkcm.configName: configData,
		}
	}

	return map[string]map[string]string{}
}

// getAllConfigs returns all configs from all servers as a single shared ConfigMap
func (gkcm *GatewayKubernetesConfigManager) getAllConfigs() map[string]map[string]string {
	allConfigData := make(map[string]string)

	// Iterate through all server configs and collect their environment variables
	for serverName, serverConfig := range gkcm.serverConfigs {
		if len(serverConfig.Spec.Env) == 0 {
			continue // Skip servers with no environment variables
		}

		if gkcm.isDockerEngine {
			// Docker-engine mode: Resolve all templates and include all environment variables
			// Use ConfigResolver to get resolved environment values for this server
			resolvedEnv := gkcm.configResolver.ResolveEnvironment(serverName)

			// Map resolved environment variables to Kubernetes ConfigMap data structure
			for _, env := range serverConfig.Spec.Env {
				if envValue, exists := resolvedEnv[env.Name]; exists {
					// Use the environment variable name as the ConfigMap key
					allConfigData[env.Name] = envValue
				}
			}
		} else {
			// Cluster mode: Only include non-templated (static) environment variables
			// Templated variables are expected to be provided out-of-band in pre-existing ConfigMaps
			for _, env := range serverConfig.Spec.Env {
				if strings.Contains(env.Value, "{{") && strings.Contains(env.Value, "}}") {
					// Skip templated values - expect them in pre-existing ConfigMaps
					continue
				}
				// Only include static (non-templated) values in our ConfigMaps
				if env.Value != "" {
					allConfigData[env.Name] = env.Value
				}
			}
		}
	}

	// Only return the shared ConfigMap if it has data
	if len(allConfigData) > 0 {
		return map[string]map[string]string{
			gkcm.configName: allConfigData,
		}
	}

	return map[string]map[string]string{}
}

// GetConfigMapRefs returns ConfigMap names for envFrom injection
func (gkcm *GatewayKubernetesConfigManager) GetConfigMapRefs(serverName string) []string {
	// Check if this server has any non-secret environment variables
	configSpecs := gkcm.GetConfigSpecs(serverName)
	if len(configSpecs) > 0 {
		return []string{gkcm.configName}
	}

	return []string{}
}
