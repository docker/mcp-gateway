package provisioners

import (
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/eval"
)

// AdaptServerConfigToSpec converts a catalog.ServerConfig to a ProvisionerSpec.
// This adapter function decouples the provisioner interface from catalog structures,
// allowing them to evolve independently while maintaining stable interfaces.
func AdaptServerConfigToSpec(serverConfig *catalog.ServerConfig, provisionerType ProvisionerType) (ProvisionerSpec, error) {
	if serverConfig == nil {
		return ProvisionerSpec{}, fmt.Errorf("serverConfig cannot be nil")
	}

	spec := ProvisionerSpec{
		Name:           serverConfig.Name,
		Image:          serverConfig.Spec.Image,
		Command:        serverConfig.Spec.Command, // Raw command with templates - resolved just-in-time by provisioner
		DisableNetwork: serverConfig.Spec.DisableNetwork,
		AllowHosts:     serverConfig.Spec.AllowHosts, // Network access control configuration
		LongLived:      serverConfig.Spec.LongLived,
	}

	// Environment variables with raw templates (secret values resolved just-in-time)
	spec.Environment = resolveTemplatedEnvironmentVars(serverConfig)

	// Note: Secret VALUES are not resolved here - only templated references are included.
	// The provisioner resolves templates like {{secret.name}} to actual values just-in-time
	// via ConfigResolver to maintain proper separation between client and provisioner sides

	// Volume resolution with template expansion
	spec.Volumes = resolveVolumes(serverConfig)

	// Resource limits extraction
	spec.Resources = extractResourceLimits(serverConfig)

	// Networks (defaults to empty, filled by provisioner)
	spec.Networks = []string{}

	// Port mappings (defaults to empty, can be extended for HTTP transport)
	spec.Ports = []PortMapping{}

	// Apply provisioner-specific transformations
	switch provisionerType {
	case DockerProvisioner:
		return adaptForDocker(spec, serverConfig)
	case KubernetesProvisioner:
		return adaptForKubernetes(spec, serverConfig)
	case CloudProvisioner:
		return adaptForCloud(spec, serverConfig)
	default:
		return ProvisionerSpec{}, fmt.Errorf("unsupported provisioner type: %v", provisionerType)
	}
}

// adaptForDocker applies Docker-specific transformations to the spec
func adaptForDocker(spec ProvisionerSpec, _ *catalog.ServerConfig) (ProvisionerSpec, error) {
	// Docker provisioner uses the spec as-is for most cases
	// Future enhancements might include Docker-specific port handling
	return spec, nil
}

// adaptForKubernetes applies Kubernetes-specific transformations to the spec
func adaptForKubernetes(spec ProvisionerSpec, _ *catalog.ServerConfig) (ProvisionerSpec, error) {
	// Kubernetes provisioner uses the spec as-is for most cases
	// Future enhancements might include Kubernetes-specific resource handling
	return spec, nil
}

// adaptForCloud applies Cloud-specific transformations to the spec
func adaptForCloud(_ ProvisionerSpec, _ *catalog.ServerConfig) (ProvisionerSpec, error) {
	return ProvisionerSpec{}, fmt.Errorf("cloud provisioner not yet implemented")
}

// resolveTemplatedEnvironmentVars includes environment variables with raw templates.
// Secret values in templates are resolved just-in-time by the provisioner.
func resolveTemplatedEnvironmentVars(serverConfig *catalog.ServerConfig) map[string]string {
	envMap := make(map[string]string)

	for _, envVar := range serverConfig.Spec.Env {
		// Include raw environment variables with templates
		// Secret templates will be resolved just-in-time by the provisioner
		if envVar.Value != "" {
			envMap[envVar.Name] = envVar.Value
		}
	}

	return envMap
}

// resolveSecrets extracts resolved secret values from server configuration
func resolveSecrets(serverConfig *catalog.ServerConfig) map[string]string {
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

// resolveVolumes processes volume mounts with template evaluation
func resolveVolumes(serverConfig *catalog.ServerConfig) []string {
	// Use eval package to resolve volume templates
	resolvedVolumes := eval.EvaluateList(serverConfig.Spec.Volumes, serverConfig.Config)

	// Filter out empty volumes
	filteredVolumes := make([]string, 0, len(resolvedVolumes))
	for _, volume := range resolvedVolumes {
		if volume != "" {
			filteredVolumes = append(filteredVolumes, volume)
		}
	}

	return filteredVolumes
}

// extractResourceLimits extracts resource limits from server configuration
func extractResourceLimits(_ *catalog.ServerConfig) ResourceLimits {
	limits := ResourceLimits{}

	// Resource limits would typically come from server config or global settings
	// For now, return empty limits since they're handled at the clientPool level
	// This function is a placeholder for future resource limit configuration

	return limits
}
