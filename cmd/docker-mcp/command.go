package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/backup"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/client"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/config"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/oauth"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/policy"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/server"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/tools"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/version"
)

// Note: We use a custom help template to make it more brief.
const helpTemplate = `Docker MCP Toolkit's CLI - Manage your MCP servers and clients.
{{if .UseLine}}
Usage: {{.UseLine}}
{{end}}{{if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}{{if .HasAvailableSubCommands}}
Available Commands:
{{range .Commands}}{{if (or .IsAvailableCommand)}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}
`

// rootCommand returns the root command for the init plugin
func rootCommand(ctx context.Context, cwd string, docker docker.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:              "mcp [OPTIONS]",
		TraverseChildren: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: false,
			HiddenDefaultCmd:  true,
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cmd.SetContext(ctx)
			if err := plugin.PersistentPreRunE(cmd, args); err != nil {
				return err
			}

			if os.Getenv("DOCKER_MCP_IN_CONTAINER") != "1" {
				return desktop.CheckFeatureIsEnabled(ctx, "enableDockerMCPToolkit", "Docker MCP Toolkit")
			}

			return nil
		},
		Version: version.Version,
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.Flags().BoolP("version", "v", false, "Print version information and quit")
	cmd.SetHelpTemplate(helpTemplate)

	_ = cmd.RegisterFlagCompletionFunc("mcp", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"--help"}, cobra.ShellCompDirectiveNoFileComp
	})

	cmd.AddCommand(secret.NewSecretsCmd(docker))
	cmd.AddCommand(policy.NewPolicyCmd())
	cmd.AddCommand(oauth.NewOAuthCmd())
	cmd.AddCommand(client.NewClientCmd(cwd))
	cmd.AddCommand(catalog.NewCatalogCmd())
	cmd.AddCommand(versionCommand())
	cmd.AddCommand(gatewayCommand(docker))
	cmd.AddCommand(configCommand(docker))
	cmd.AddCommand(serverCommand(docker))
	cmd.AddCommand(toolsCommand())

	if os.Getenv("DOCKER_MCP_SHOW_HIDDEN") == "1" {
		unhideHiddenCommands(cmd)
	}

	return cmd
}

func unhideHiddenCommands(cmd *cobra.Command) {
	// Unhide all commands that are marked as hidden
	for _, c := range cmd.Commands() {
		c.Hidden = false
		unhideHiddenCommands(c)
	}
}

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Short: "Show the version information",
		Use:   "version",
		Args:  cobra.ExactArgs(0),
		// Deactivate PersistentPreRun for this command only
		// We don't want to check if Docker Desktop is running.
		PersistentPreRun: func(*cobra.Command, []string) {},
		Run: func(*cobra.Command, []string) {
			fmt.Println(version.Version)
		},
	}
}

func gatewayCommand(docker docker.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Manage the MCP Server gateway",
	}

	// Have different defaults for the on-host gateway and the in-container gateway.
	var options gateway.Config
	if os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1" {
		// In-container.
		options = gateway.Config{
			CatalogPath: catalog.DockerCatalogURL,
			SecretsPath: "docker-desktop:/run/secrets/mcp_secret:/.env",
			Options: gateway.Options{
				Cpus:             1,
				Memory:           "2Gb",
				Transport:        "stdio",
				Port:             8811,
				LogCalls:         true,
				BlockSecrets:     true,
				VerifySignatures: true,
				Verbose:          true,
			},
		}
	} else {
		// On-host.
		options = gateway.Config{
			CatalogPath:  "docker-mcp.yaml",
			RegistryPath: "registry.yaml",
			ConfigPath:   "config.yaml",
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
			return gateway.NewGateway(options, docker).Run(cmd.Context())
		},
	}

	runCmd.Flags().StringSliceVar(&options.ServerNames, "servers", nil, "names of the servers to enable (if non empty, ignore --registry flag)")
	runCmd.Flags().StringVar(&options.CatalogPath, "catalog", options.CatalogPath, "path to the docker-mcp.yaml catalog (absolute or relative to ~/.docker/mcp/catalogs/)")
	runCmd.Flags().StringVar(&options.RegistryPath, "registry", options.RegistryPath, "path to the registry.yaml (absolute or relative to ~/.docker/mcp/)")
	runCmd.Flags().StringVar(&options.ConfigPath, "config", options.ConfigPath, "path to the config.yaml (absolute or relative to ~/.docker/mcp/)")
	runCmd.Flags().StringVar(&options.SecretsPath, "secrets", options.SecretsPath, "colon separated paths to search for secrets. Can be `docker-desktop` or a path to a .env file (default to using Docker Deskop's secrets API)")
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
	runCmd.Flags().BoolVar(&options.AllSingletons, "all-singletons", options.AllSingletons, "Containers are singletons that will be reused")
	runCmd.Flags().BoolVar(&options.DebugDNS, "debug-dns", options.DebugDNS, "Debug DNS resolution")
	runCmd.Flags().BoolVar(&options.Watch, "watch", options.Watch, "Watch for changes and reconfigure the gateway")
	runCmd.Flags().IntVar(&options.Cpus, "cpus", options.Cpus, "CPUs allocated to each MCP Server (default is 1)")
	runCmd.Flags().StringVar(&options.Memory, "memory", options.Memory, "Memory allocated to each MCP Server (default is 2Gb)")
	runCmd.Flags().BoolVar(&options.Static, "static", options.Static, "Enable static mode (aka pre-started servers)")

	cmd.AddCommand(runCmd)

	return cmd
}

// TODO(dga): Those commands are a first step to delegating the work to the CLI.
// names and hierarchy are not final.
func configCommand(docker docker.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "read",
		Short: "Read the configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			content, err := config.ReadConfig(cmd.Context(), docker)
			if err != nil {
				return err
			}
			_, _ = cmd.OutOrStdout().Write(content)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "write",
		Short: "Write the configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return config.WriteConfig([]byte(args[0]))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset the configuration",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return config.WriteConfig(nil)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:    "dump",
		Short:  "Dump the whole configuration",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out, err := backup.Dump(cmd.Context(), docker)
			if err != nil {
				return err
			}
			_, _ = cmd.OutOrStdout().Write(out)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:    "restore",
		Short:  "Restore the whole configuration",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			var backupData []byte
			if path == "-" {
				var err error
				backupData, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading from stdin: %w", err)
				}
			} else {
				var err error
				backupData, err = os.ReadFile(path)
				if err != nil {
					return err
				}
			}

			return backup.Restore(cmd.Context(), backupData)
		},
	})

	return cmd
}

// TODO(dga): Those commands are a first step to delegating the work to the CLI.
// names and hierarchy are not final.
func serverCommand(docker docker.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers",
	}

	var outputJSON bool
	lsCommand := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list enabled servers",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			list, err := server.List(cmd.Context(), docker)
			if err != nil {
				return err
			}

			if outputJSON {
				buf, err := json.Marshal(list)
				if err != nil {
					return err
				}
				_, _ = cmd.OutOrStdout().Write(buf)
			} else if len(list) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No server is enabled")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(list, ", "))
			}

			return nil
		},
		Hidden: true,
	}
	lsCommand.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")
	cmd.AddCommand(lsCommand)

	cmd.AddCommand(&cobra.Command{
		Use:     "enable",
		Aliases: []string{"add"},
		Short:   "Enable a server or multiple servers",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.Enable(cmd.Context(), docker, args)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:     "disable",
		Aliases: []string{"remove", "rm"},
		Short:   "Disable a server or multiple servers",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.Disable(cmd.Context(), docker, args)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "inspect",
		Short: "Get information about a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := server.Inspect(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			buf, err := info.ToJSON()
			if err != nil {
				return err
			}

			_, _ = cmd.OutOrStdout().Write(buf)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Disable all the servers",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return config.WriteRegistry(nil)
		},
	})

	return cmd
}

func toolsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List/count/call MCP tools",
	}

	var version string
	var verbose bool
	var format string
	var gatewayArgs []string
	cmd.PersistentFlags().StringVar(&version, "version", "2", "Version of the gateway")
	cmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Verbose output")
	cmd.PersistentFlags().StringVar(&format, "format", "list", "Output format (json|list)")
	cmd.PersistentFlags().StringSliceVar(&gatewayArgs, "gateway-arg", nil, "Additional arguments passed to the gateway")

	cmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list tools",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tools.List(cmd.Context(), version, gatewayArgs, verbose, "list", "", format)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "count",
		Short: "count tools",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tools.List(cmd.Context(), version, gatewayArgs, verbose, "count", "", format)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "inspect",
		Short: "inspect a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return tools.List(cmd.Context(), version, gatewayArgs, verbose, "inspect", args[0], format)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "call",
		Short: "call a tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tools.Call(cmd.Context(), version, gatewayArgs, verbose, args)
		},
	})

	return cmd
}
