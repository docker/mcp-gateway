package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	catalogTypes "github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/gateway"
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
	var mcpRegistryUrls []string
	var enableAllServers bool
	if os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1" {
		// In-container.
		// Note: The catalog URL will be updated after checking the feature flag in RunE
		options = gateway.Config{
			CatalogPath: []string{catalog.DockerCatalogURLV2}, // Default to v2, will be updated based on flag
			SecretsPath: "docker-desktop:/run/secrets/mcp_secret:/.env",
			Options: gateway.Options{
				Cpus:         1,
				Memory:       "2Gb",
				Transport:    "stdio",
				LogCalls:     true,
				BlockSecrets: true,
				Verbose:      true,
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
				Cpus:         1,
				Memory:       "2Gb",
				Transport:    "stdio",
				LogCalls:     true,
				BlockSecrets: true,
				Watch:        true,
			},
		}
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the gateway",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Check if OAuth interceptor feature is enabled
			options.OAuthInterceptorEnabled = isOAuthInterceptorFeatureEnabled(dockerCli)

			// Check if MCP OAuth DCR feature is enabled
			options.McpOAuthDcrEnabled = isMcpOAuthDcrFeatureEnabled(dockerCli)

			// Check if dynamic tools feature is enabled
			options.DynamicTools = isDynamicToolsFeatureEnabled(dockerCli)

			// Check if tool name prefix feature is enabled
			options.ToolNamePrefix = isToolNamePrefixFeatureEnabled(dockerCli)

			// Update catalog URL based on mcp-oauth-dcr flag if using default Docker catalog URL
			if len(options.CatalogPath) == 1 && (options.CatalogPath[0] == catalog.DockerCatalogURLV2 || options.CatalogPath[0] == catalog.DockerCatalogURLV3) {
				options.CatalogPath[0] = catalog.GetDockerCatalogURL(options.McpOAuthDcrEnabled)
			}

			if options.Static {
				options.Watch = false
			}

			if options.Transport == "stdio" {
				if options.Port != 0 {
					return errors.New("cannot use --port with --transport=stdio")
				}
			} else if options.Port == 0 {
				options.Port = 8811
			}

			// Build catalog path list with proper precedence order and no duplicates
			defaultPaths := convertCatalogNamesToPaths(options.CatalogPath) // Convert any catalog names to paths

			// Only add configured catalogs if defaultPaths is not a single Docker catalog entry
			var configuredPaths []string
			if len(defaultPaths) == 1 && (defaultPaths[0] == catalog.DockerCatalogURLV2 || defaultPaths[0] == catalog.DockerCatalogURLV3 || defaultPaths[0] == catalog.DockerCatalogFilename) {
				configuredPaths = getConfiguredCatalogPaths()
			}
			catalogPaths := buildUniqueCatalogPaths(defaultPaths, configuredPaths, additionalCatalogs)
			options.CatalogPath = catalogPaths

			options.RegistryPath = append(options.RegistryPath, additionalRegistries...)
			options.ConfigPath = append(options.ConfigPath, additionalConfigs...)
			options.ToolsPath = append(options.ToolsPath, additionalToolsConfig...)

			// Process MCP registry URLs if provided
			if len(mcpRegistryUrls) > 0 {
				var mcpServers []catalogTypes.Server
				for _, registryURL := range mcpRegistryUrls {
					if err := runMcpregistryImport(cmd.Context(), registryURL, &mcpServers); err != nil {
						return fmt.Errorf("failed to fetch server from MCP registry %s: %w", registryURL, err)
					}
				}
				options.MCPRegistryServers = mcpServers
			}

			// Handle --enable-all-servers flag
			if enableAllServers {
				if len(options.ServerNames) > 0 {
					return fmt.Errorf("cannot use --enable-all-servers with --servers flag")
				}

				// Read all catalogs to get server names
				mcpCatalog, err := catalogTypes.ReadFrom(cmd.Context(), catalogPaths)
				if err != nil {
					return fmt.Errorf("failed to read catalogs for --enable-all-servers: %w", err)
				}

				// Extract all server names from the catalog
				var allServerNames []string
				for serverName := range mcpCatalog.Servers {
					allServerNames = append(allServerNames, serverName)
				}
				options.ServerNames = allServerNames
			}

			return gateway.NewGateway(options, docker).Run(cmd.Context())
		},
	}

	runCmd.Flags().StringSliceVar(&options.ServerNames, "servers", nil, "Names of the servers to enable (if non empty, ignore --registry flag)")
	runCmd.Flags().BoolVar(&enableAllServers, "enable-all-servers", false, "Enable all servers in the catalog (instead of using individual --servers options)")
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
	runCmd.Flags().StringArrayVar(&options.OciRef, "oci-ref", options.OciRef, "OCI image references to use")
	runCmd.Flags().StringSliceVar(&mcpRegistryUrls, "mcp-registry", nil, "MCP registry URLs to fetch servers from (can be repeated)")
	runCmd.Flags().IntVar(&options.Port, "port", options.Port, "TCP port to listen on (default is to listen on stdio)")
	runCmd.Flags().StringVar(&options.Transport, "transport", options.Transport, "stdio, sse or streaming. Uses MCP_GATEWAY_AUTH_TOKEN environment variable for localhost authentication to prevent dns rebinding attacks.")
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
	runCmd.Flags().BoolVar(&options.Static, "static", options.Static, "Enable static mode (aka pre-started servers)")
	runCmd.Flags().StringVar(&options.LogFilePath, "log", options.LogFilePath, "Path to log file for stderr output (relative or absolute)")

	// Very experimental features
	_ = runCmd.Flags().MarkHidden("log")

	cmd.AddCommand(runCmd)

	return cmd
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

// convertCatalogNamesToPaths converts catalog names to their corresponding file paths.
// If a path entry is already a file path (contains .yaml or is an absolute/relative path),
// it's returned as-is. If it's a catalog name from the configuration, it's converted to catalogName.yaml.
func convertCatalogNamesToPaths(catalogPaths []string) []string {
	cfg, err := catalog.ReadConfig()
	if err != nil {
		// If config doesn't exist or can't be read, return paths as-is
		return catalogPaths
	}

	var result []string
	for _, path := range catalogPaths {
		// If it's already a file path (contains .yaml, starts with /, ./, or ../), keep as-is
		if strings.Contains(path, ".yaml") || strings.HasPrefix(path, "/") ||
			strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
			result = append(result, path)
		} else if _, exists := cfg.Catalogs[path]; exists {
			// It's a catalog name from config, convert to file path
			result = append(result, path+".yaml")
		} else {
			// Not a known catalog name and not a clear file path, keep as-is
			result = append(result, path)
		}
	}

	return result
}

// buildUniqueCatalogPaths builds a unique list of catalog paths with proper precedence order:
// 1. Default catalogs (e.g., docker-mcp.yaml)
// 2. Configured catalogs (from catalog management system)
// 3. Additional catalogs (CLI-specified, highest precedence)
// Duplicates are removed while preserving order and precedence.
func buildUniqueCatalogPaths(defaultPaths, configuredPaths, additionalPaths []string) []string {
	seen := make(map[string]bool)
	var result []string

	// Helper function to add unique paths
	addUnique := func(paths []string) {
		for _, path := range paths {
			if !seen[path] {
				seen[path] = true
				result = append(result, path)
			}
		}
	}

	// Add paths in precedence order
	addUnique(defaultPaths)
	addUnique(configuredPaths)
	addUnique(additionalPaths)

	return result
}

// isOAuthInterceptorFeatureEnabled checks if the oauth-interceptor feature is enabled
func isOAuthInterceptorFeatureEnabled(dockerCli command.Cli) bool {
	configFile := dockerCli.ConfigFile()
	if configFile == nil || configFile.Features == nil {
		return false
	}

	value, exists := configFile.Features["oauth-interceptor"]
	if !exists {
		return false
	}

	return value == "enabled"
}

// isMcpOAuthDcrFeatureEnabled checks if the mcp-oauth-dcr feature is enabled
func isMcpOAuthDcrFeatureEnabled(dockerCli command.Cli) bool {
	configFile := dockerCli.ConfigFile()
	if configFile == nil || configFile.Features == nil {
		return true // Default enabled when no config exists
	}

	value, exists := configFile.Features["mcp-oauth-dcr"]
	if !exists {
		return true // Default enabled when not in config
	}

	return value == "enabled"
}

// isDynamicToolsFeatureEnabled checks if the dynamic-tools feature is enabled
func isDynamicToolsFeatureEnabled(dockerCli command.Cli) bool {
	configFile := dockerCli.ConfigFile()
	if configFile == nil || configFile.Features == nil {
		return false
	}

	value, exists := configFile.Features["dynamic-tools"]
	if !exists {
		return false
	}

	return value == "enabled"
}

// isToolNamePrefixFeatureEnabled checks if the tool-name-prefix feature is enabled
func isToolNamePrefixFeatureEnabled(dockerCli command.Cli) bool {
	configFile := dockerCli.ConfigFile()
	if configFile == nil || configFile.Features == nil {
		return false
	}

	value, exists := configFile.Features["tool-name-prefix"]
	if !exists {
		return false
	}

	return value == "enabled"
}
