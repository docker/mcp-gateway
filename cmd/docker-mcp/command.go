package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/docker/mcp-cli/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-cli/cmd/docker-mcp/client"
	"github.com/docker/mcp-cli/cmd/docker-mcp/internal/config"
	"github.com/docker/mcp-cli/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-cli/cmd/docker-mcp/internal/gateway"
	"github.com/docker/mcp-cli/cmd/docker-mcp/oauth"
	"github.com/docker/mcp-cli/cmd/docker-mcp/secret-management/policy"
	"github.com/docker/mcp-cli/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-cli/cmd/docker-mcp/server"
	"github.com/docker/mcp-cli/cmd/docker-mcp/tools"
	"github.com/docker/mcp-cli/cmd/docker-mcp/version"
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
func rootCommand(ctx context.Context, cwd string, dockerCli command.Cli) *cobra.Command {
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

	cmd.AddCommand(secret.NewSecretsCmd())
	cmd.AddCommand(policy.NewPolicyCmd())
	cmd.AddCommand(oauth.NewOAuthCmd())
	cmd.AddCommand(client.NewClientCmd(cwd))
	cmd.AddCommand(catalog.NewCatalogCmd())
	cmd.AddCommand(gatewayCommand(dockerCli))
	cmd.AddCommand(configCommand(dockerCli))
	cmd.AddCommand(serverCommand(dockerCli))
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

func gatewayCommand(dockerCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Manage the MCP Server gateway",
	}

	options := gateway.Config{
		CatalogPath:  "docker-mcp.yaml",
		RegistryPath: "registry.yaml",
		ConfigPath:   "config.yaml",
		Options: gateway.Options{
			Transport:    "stdio",
			LogCalls:     true,
			BlockSecrets: true,
		},
	}
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the gateway",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return gateway.NewGateway(options, dockerCli).Run(cmd.Context())
		},
	}

	runCmd.Flags().StringArrayVar(&options.ServerNames, "servers", nil, "names of the servers to enable (if non empty, ignore --registry flag)")
	runCmd.Flags().StringVar(&options.CatalogPath, "catalog", options.CatalogPath, "path to the docker-mcp.yaml catalog (absolute or relative to ~/.docker/mcp/catalogs/)")
	runCmd.Flags().StringVar(&options.RegistryPath, "registry", options.RegistryPath, "path to the registry.yaml (absolute or relative to ~/.docker/mcp/)")
	runCmd.Flags().StringVar(&options.ConfigPath, "config", options.ConfigPath, "path to the config.yaml (absolute or relative to ~/.docker/mcp/)")
	runCmd.Flags().StringVar(&options.SecretsPath, "secrets", "", "path to a .env file containing secrets (default to using Docker Deskop's secrets API)")
	runCmd.Flags().StringArrayVar(&options.ToolNames, "tools", options.ToolNames, "List of tools to enable")
	runCmd.Flags().IntVar(&options.Port, "port", options.Port, "TCP port to listen on (default is to listen on stdio)")
	runCmd.Flags().StringVar(&options.Transport, "transport", options.Transport, "stdio, sse or streaming (default is stdio)")
	runCmd.Flags().BoolVar(&options.LogCalls, "log-calls", options.LogCalls, "Log calls to the tools")
	runCmd.Flags().BoolVar(&options.BlockSecrets, "block-secrets", options.BlockSecrets, "Block secrets from being/received sent to/from tools")
	runCmd.Flags().BoolVar(&options.VerifySignatures, "verify-signatures", options.VerifySignatures, "Verify signatures of the server images")
	runCmd.Flags().BoolVar(&options.DryRun, "dry-run", options.DryRun, "Start the gateway but do not listen for connections (useful for testing the configuration)")
	runCmd.Flags().BoolVar(&options.Verbose, "verbose", options.Verbose, "Verbose output")
	runCmd.Flags().BoolVar(&options.KeepContainers, "keep", options.KeepContainers, "Keep stopped containers")
	runCmd.Flags().BoolVar(&options.Watch, "watch", options.Watch, "Watch for changes and reconfigure the gateway")

	cmd.AddCommand(runCmd)

	return cmd
}

// TODO(dga): Those commands are a first step to delegating the work to the CLI.
// names and hierarchy are not final.
func configCommand(dockerCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "read",
		Short: "Read the configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			content, err := config.ReadConfig(cmd.Context(), dockerCli.Client())
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

	return cmd
}

// TODO(dga): Those commands are a first step to delegating the work to the CLI.
// names and hierarchy are not final.
func serverCommand(dockerCli command.Cli) *cobra.Command {
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
			list, err := server.List(cmd.Context(), dockerCli.Client())
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
			return server.Enable(cmd.Context(), dockerCli.Client(), args)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:     "disable",
		Aliases: []string{"remove", "rm"},
		Short:   "Disable a server or multiple servers",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.Disable(cmd.Context(), dockerCli.Client(), args)
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
	cmd.PersistentFlags().StringVar(&version, "version", "2", "Version of the gateway")
	cmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Verbose output")
	cmd.PersistentFlags().StringVar(&format, "format", "list", "Output format (json|list)")

	cmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list tools",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tools.List(cmd.Context(), version, verbose, "list", "", format)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "count",
		Short: "count tools",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tools.List(cmd.Context(), version, verbose, "count", "", format)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "inspect",
		Short: "inspect a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return tools.List(cmd.Context(), version, verbose, "inspect", args[0], format)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "call",
		Short: "call a tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tools.Call(cmd.Context(), version, verbose, args)
		},
	})

	return cmd
}
