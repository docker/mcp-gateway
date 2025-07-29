package commands

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway"
)

func gatewayCommand(docker docker.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Manage the MCP Server gateway",
	}

	// Have different defaults for the on-host gateway and the in-container gateway.
	var options gateway.Config
	var additionalCatalogs []string
	var additionalRegistries []string
	var additionalConfigs []string
	if os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1" {
		// In-container.
		options = gateway.Config{
			CatalogPath: []string{catalog.DockerCatalogURL},
			SecretsPath: "docker-desktop:/run/secrets/mcp_secret:/.env",
			Options: gateway.Options{
				Cpus:             1,
				Memory:           "2Gb",
				Transport:        "stdio",
				LogCalls:         true,
				BlockSecrets:     true,
				VerifySignatures: true,
				Verbose:          true,
			},
		}
	} else {
		// On-host.
		options = gateway.Config{
			CatalogPath:  []string{"docker-mcp.yaml"},
			RegistryPath: []string{"registry.yaml"},
			ConfigPath:   []string{"config.yaml"},
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

			// Append additional catalogs to the main catalog path
			options.CatalogPath = append(options.CatalogPath, additionalCatalogs...)
			options.RegistryPath = append(options.RegistryPath, additionalRegistries...)
			options.ConfigPath = append(options.ConfigPath, additionalConfigs...)

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
	runCmd.Flags().BoolVar(&options.Static, "static", options.Static, "Enable static mode (aka pre-started servers)")

	// Very experimental features
	runCmd.Flags().BoolVar(&options.Central, "central", options.Central, "In central mode, clients tell us which servers to enable")
	_ = runCmd.Flags().MarkHidden("central")

	cmd.AddCommand(runCmd)

	return cmd
}
