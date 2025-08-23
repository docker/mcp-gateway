package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/provisioners"
)

func gatewayCommand(docker docker.Client, dockerCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Manage the MCP Server gateway",
	}

	// Have different defaults for the on-host gateway and the in-container gateway.
	var options gateway.Config
	var additionalCatalogs []string
	var additionalRegistries []string
	var additionalConfigs []string
	var additionalToolsConfig []string
	var useConfiguredCatalogs bool
	var secretProviderStr string // For CLI flag parsing
	var configProviderStr string // For CLI flag parsing

	if os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1" {
		// In-container.
		options = gateway.Config{
			CatalogPath: []string{catalog.DockerCatalogURL},
			SecretsPath: "docker-desktop:/run/secrets/mcp_secret:/.env",
			Options: gateway.Options{
				Cpus:                    1,
				Memory:                  "2Gb",
				MaxServerStartupTimeout: 10,
				Transport:               "stdio",
				LogCalls:                true,
				BlockSecrets:            true,
				VerifySignatures:        true,
				Verbose:                 false, // Default to quiet unless --verbose flag is used
				Provisioner:             "docker",
				SecretProvider:          provisioners.DockerEngineSecretProvider,
				SecretName:              "mcp-gateway-secrets",
				ConfigProvider:          provisioners.DockerEngineConfigProvider,
				ConfigName:              "mcp-gateway-config",
			},
		}
	} else {
		// On-host.
		options = gateway.Config{
			CatalogPath:  []string{catalog.DockerCatalogFilename},
			RegistryPath: []string{"registry.yaml"},
			ConfigPath:   []string{"config.yaml"},
			ToolsPath:    []string{"tools.yaml"},
			SecretsPath:  "docker-desktop",
			Options: gateway.Options{
				Cpus:                    1,
				Memory:                  "2Gb",
				MaxServerStartupTimeout: 10,
				Transport:               "stdio",
				LogCalls:                true,
				BlockSecrets:            true,
				Watch:                   true,
				Provisioner:             "docker",
				SecretProvider:          provisioners.DockerEngineSecretProvider,
				SecretName:              "mcp-gateway-secrets",
				ConfigProvider:          provisioners.DockerEngineConfigProvider,
				ConfigName:              "mcp-gateway-config",
			},
		}
	}

	// Initialize string representation for flag parsing
	secretProviderStr = options.SecretProvider.String()
	configProviderStr = options.ConfigProvider.String()

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the gateway",
		Args:  cobra.NoArgs,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			// Validate provisioner selection
			if err := validateProvisioner(options.Provisioner); err != nil {
				return err
			}

			// Validate kubernetes-provisioning feature flag
			if err := validateKubernetesProvisioningFeatureForCli(dockerCli, options.Provisioner); err != nil {
				return err
			}

			// Parse and validate secret provider selection
			secretProviderType, err := provisioners.ParseSecretProviderType(secretProviderStr)
			if err != nil {
				return err
			}
			options.SecretProvider = secretProviderType

			// Parse and validate config provider selection
			configProviderType, err := provisioners.ParseConfigProviderType(configProviderStr)
			if err != nil {
				return err
			}
			options.ConfigProvider = configProviderType

			// Validate configured catalogs feature flag
			return validateConfiguredCatalogsFeatureForCli(dockerCli, useConfiguredCatalogs)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Check if OAuth interceptor feature is enabled
			options.OAuthInterceptorEnabled = isOAuthInterceptorFeatureEnabled(dockerCli)

			if options.Static {
				options.Watch = false
			}

			if options.Central {
				options.Watch = false
				options.Transport = "streaming"
			}

			if options.Transport == "stdio" {
				if options.Port != 0 {
					return errors.New("cannot use --port with --transport=stdio")
				}
			} else if options.Port == 0 {
				options.Port = 8811
			}

			// Build catalog path list with proper precedence order
			catalogPaths := options.CatalogPath // Start with existing catalog paths (includes docker-mcp.yaml default)

			// Add configured catalogs if requested
			if useConfiguredCatalogs {
				configuredPaths := getConfiguredCatalogPaths()
				// Insert configured catalogs after docker-mcp.yaml but before CLI-specified catalogs
				if len(catalogPaths) > 0 {
					// Insert after the first element (docker-mcp.yaml)
					catalogPaths = append(catalogPaths[:1], append(configuredPaths, catalogPaths[1:]...)...)
				} else {
					catalogPaths = append(catalogPaths, configuredPaths...)
				}
			}

			// Append additional catalogs (CLI-specified have highest precedence)
			catalogPaths = append(catalogPaths, additionalCatalogs...)
			options.CatalogPath = catalogPaths

			options.RegistryPath = append(options.RegistryPath, additionalRegistries...)
			options.ConfigPath = append(options.ConfigPath, additionalConfigs...)
			options.ToolsPath = append(options.ToolsPath, additionalToolsConfig...)

			return gateway.NewGateway(options, docker).Run(cmd.Context())
		},
	}

	runCmd.Flags().StringSliceVar(&options.ServerNames, "servers", nil, "Names of the servers to enable (if non empty, ignore --registry flag)")
	runCmd.Flags().StringSliceVar(&options.CatalogPath, "catalog", options.CatalogPath, "Paths to docker catalogs (absolute or relative to ~/.docker/mcp/catalogs/)")
	runCmd.Flags().StringSliceVar(&additionalCatalogs, "additional-catalog", nil, "Additional catalog paths to append to the default catalogs")
	runCmd.Flags().StringSliceVar(&options.RegistryPath, "registry", options.RegistryPath, "Paths to the registry files (absolute or relative to ~/.docker/mcp/)")
	runCmd.Flags().StringSliceVar(&additionalRegistries, "additional-registry", nil, "Additional registry paths to merge with the default registry.yaml")
	runCmd.Flags().StringSliceVar(&options.ConfigPath, "config", options.ConfigPath, "Paths to the config files (absolute or relative to ~/.docker/mcp/)")
	runCmd.Flags().StringSliceVar(&additionalConfigs, "additional-config", nil, "Additional config paths to merge with the default config.yaml")
	runCmd.Flags().StringSliceVar(&options.ToolsPath, "tools-config", options.ToolsPath, "Paths to the tools files (absolute or relative to ~/.docker/mcp/)")
	runCmd.Flags().StringSliceVar(&additionalToolsConfig, "additional-tools-config", nil, "Additional tools paths to merge with the default tools.yaml")
	runCmd.Flags().StringVar(&options.SecretsPath, "secrets", options.SecretsPath, "Colon separated paths to search for secrets. Can be `docker-desktop` or a path to a .env file (default to using Docker Desktop's secrets API)")
	runCmd.Flags().StringSliceVar(&options.ToolNames, "tools", options.ToolNames, "List of tools to enable")
	runCmd.Flags().StringArrayVar(&options.Interceptors, "interceptor", options.Interceptors, "List of interceptors to use (format: when:type:path, e.g. 'before:exec:/bin/path')")
	runCmd.Flags().IntVar(&options.Port, "port", options.Port, "TCP port to listen on (default is to listen on stdio)")
	runCmd.Flags().StringVar(&options.Transport, "transport", options.Transport, "stdio, sse or streaming (default is stdio)")
	runCmd.Flags().BoolVar(&options.LogCalls, "log-calls", options.LogCalls, "Log calls to the tools")
	runCmd.Flags().BoolVar(&options.BlockSecrets, "block-secrets", options.BlockSecrets, "Block secrets from being/received sent to/from tools")
	runCmd.Flags().BoolVar(&options.BlockNetwork, "block-network", options.BlockNetwork, "Block tools from accessing forbidden network resources")
	runCmd.Flags().BoolVar(&options.VerifySignatures, "verify-signatures", options.VerifySignatures, "Verify signatures of the server images")
	runCmd.Flags().BoolVar(&options.DryRun, "dry-run", options.DryRun, "Start the gateway but do not listen for connections (useful for testing the configuration)")
	runCmd.Flags().BoolVar(&options.Verbose, "verbose", options.Verbose, "Verbose output")
	runCmd.Flags().BoolVar(&options.LongLived, "long-lived", options.LongLived, "Containers are long-lived and will not be removed until the gateway is stopped, useful for stateful servers")
	runCmd.Flags().BoolVar(&options.DebugDNS, "debug-dns", options.DebugDNS, "Debug DNS resolution")
	runCmd.Flags().BoolVar(&options.Watch, "watch", options.Watch, "Watch for changes and reconfigure the gateway")
	runCmd.Flags().IntVar(&options.Cpus, "cpus", options.Cpus, "CPUs allocated to each MCP Server (default is 1)")
	runCmd.Flags().StringVar(&options.Memory, "memory", options.Memory, "Memory allocated to each MCP Server (default is 2Gb)")
	runCmd.Flags().IntVar(&options.MaxServerStartupTimeout, "max-server-startup-timeout", options.MaxServerStartupTimeout, "Maximum time in seconds to wait for each server to start (default is 10)")
	runCmd.Flags().BoolVar(&options.Static, "static", options.Static, "Enable static mode (aka pre-started servers)")

	// Provisioner configuration
	runCmd.Flags().StringVar(&options.Provisioner, "provisioner", options.Provisioner, "Provisioner to use: docker, kubernetes (or k8s)")

	// Docker-specific flags (hidden - experimental)
	runCmd.Flags().StringVar(&options.DockerContext, "docker-context", options.DockerContext, "Docker context to use (for docker provisioner)")
	_ = runCmd.Flags().MarkHidden("docker-context")

	// Kubernetes-specific flags
	runCmd.Flags().StringVar(&options.Kubeconfig, "kubeconfig", options.Kubeconfig, "Path to kubeconfig file (for kubernetes provisioner)")
	runCmd.Flags().StringVar(&options.Namespace, "namespace", "default", "Kubernetes namespace (for kubernetes provisioner)")
	runCmd.Flags().StringVar(&options.KubeContext, "kube-context", options.KubeContext, "Kubernetes context (for kubernetes provisioner)")

	// Kubernetes cluster configuration
	runCmd.Flags().StringVar(&secretProviderStr, "cluster-secret-provider", secretProviderStr, "Kubernetes secret provider: docker-engine, cluster")
	runCmd.Flags().StringVar(&options.SecretName, "cluster-secret-name", options.SecretName, "Kubernetes Secret resource name for cluster mode (default: mcp-gateway-secrets)")
	runCmd.Flags().StringVar(&configProviderStr, "cluster-config-provider", configProviderStr, "Kubernetes config provider: docker-engine, cluster")
	runCmd.Flags().StringVar(&options.ConfigName, "cluster-config-name", options.ConfigName, "Kubernetes ConfigMap resource name for cluster mode (default: mcp-gateway-config)")

	// Configured catalogs feature
	runCmd.Flags().BoolVar(&useConfiguredCatalogs, "use-configured-catalogs", false, "Include user-managed catalogs (requires 'configured-catalogs' feature to be enabled)")

	// Very experimental features
	runCmd.Flags().BoolVar(&options.Central, "central", options.Central, "In central mode, clients tell us which servers to enable")
	_ = runCmd.Flags().MarkHidden("central")

	cmd.AddCommand(runCmd)

	return cmd
}

// validateConfiguredCatalogsFeatureForCli validates that the configured-catalogs feature is enabled when requested
func validateConfiguredCatalogsFeatureForCli(dockerCli command.Cli, useConfigured bool) error {
	if !useConfigured {
		return nil // No validation needed when feature not requested
	}

	return validateFeatureEnabled(dockerCli, "configured-catalogs", `configured catalogs feature is not enabled

To enable this experimental feature, run:
  docker mcp feature enable configured-catalogs

This feature allows the gateway to automatically include user-managed catalogs
alongside the default Docker catalog`)
}

// getConfiguredCatalogPaths returns the file paths of all configured catalogs
func getConfiguredCatalogPaths() []string {
	cfg, err := catalog.ReadConfig()
	if err != nil {
		// If config doesn't exist or can't be read, return empty list
		// This is not an error condition - user just hasn't configured any catalogs yet
		return []string{}
	}

	var catalogPaths []string
	for catalogName := range cfg.Catalogs {
		// Skip the Docker catalog as it's handled separately
		if catalogName != catalog.DockerCatalogName {
			catalogPaths = append(catalogPaths, catalogName+".yaml")
		}
	}

	return catalogPaths
}

// validateProvisioner validates the provisioner selection and provides appropriate errors
func validateProvisioner(provisioner string) error {
	switch provisioner {
	case "docker":
		return nil // Docker is fully supported
	case "k8s", "kubernetes":
		return nil // Kubernetes is now supported in Phase 4.1
	default:
		return fmt.Errorf(`invalid provisioner: %s

Supported provisioners:
  docker     - Docker container deployment (default)
  kubernetes - Kubernetes pod deployment (also: k8s)`, provisioner)
	}
}

// validateFeatureEnabled validates that a feature is enabled, with custom error message if not
func validateFeatureEnabled(dockerCli command.Cli, featureName string, errorMessage string) error {
	// Check if config is accessible (container mode check)
	configFile := dockerCli.ConfigFile()
	if configFile == nil {
		return fmt.Errorf(`docker configuration not accessible.

If running in container, mount Docker config:
  -v ~/.docker:/root/.docker

Or mount just the config file:  
  -v ~/.docker/config.json:/root/.docker/config.json`)
	}

	// Check if feature is enabled
	if configFile.Features != nil {
		if value, exists := configFile.Features[featureName]; exists {
			if value == "enabled" {
				return nil // Feature is enabled
			}
		}
	}

	// Feature not enabled - return custom error message
	return fmt.Errorf("%s", errorMessage)
}

// isFeatureEnabled checks if a feature is enabled (boolean check)
func isFeatureEnabled(dockerCli command.Cli, featureName string) bool {
	configFile := dockerCli.ConfigFile()
	if configFile == nil || configFile.Features == nil {
		return false
	}

	value, exists := configFile.Features[featureName]
	if !exists {
		return false
	}

	return value == "enabled"
}

// isOAuthInterceptorFeatureEnabled checks if the oauth-interceptor feature is enabled
func isOAuthInterceptorFeatureEnabled(dockerCli command.Cli) bool {
	return isFeatureEnabled(dockerCli, "oauth-interceptor")
}

// validateKubernetesProvisioningFeatureForCli validates that the kubernetes-provisioning feature is enabled when using k8s provisioner
func validateKubernetesProvisioningFeatureForCli(dockerCli command.Cli, provisioner string) error {
	// Only validate when kubernetes provisioner is requested
	if provisioner != "kubernetes" && provisioner != "k8s" {
		return nil // No validation needed for non-kubernetes provisioners
	}

	return validateFeatureEnabled(dockerCli, "kubernetes-provisioning", `kubernetes provisioner feature is not enabled

The Kubernetes provisioner requires enabling an experimental feature.
To enable this experimental feature, run:
  docker mcp feature enable kubernetes-provisioning

This feature allows the gateway to deploy MCP servers to Kubernetes clusters
instead of local Docker containers.`)
}
