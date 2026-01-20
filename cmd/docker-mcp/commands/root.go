package commands

import (
	"context"
	"os"
	"slices"

	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/version"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/features"
	"github.com/docker/mcp-gateway/pkg/migrate"
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

// disableCommand marks a command and all its subcommands as unavailable.
// The command is hidden and returns exit code 1 when executed.
func disableCommand(cmd *cobra.Command) *cobra.Command {
	cmd.Hidden = true

	// Override Args to always fail with our message
	cmd.Args = func(cmd *cobra.Command, args []string) error {
		cmd.PrintErrln("Error: this command is currently unavailable")
		os.Exit(1)
		return nil
	}

	// Override RunE to print error and exit
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.PrintErrln("Error: this command is currently unavailable")
		os.Exit(1)
		return nil
	}

	// Clear Run if it exists
	cmd.Run = nil

	// Recursively disable all subcommands
	for _, subCmd := range cmd.Commands() {
		disableCommand(subCmd)
	}

	return cmd
}

// Root returns the root command for the init plugin
func Root(ctx context.Context, cwd string, dockerCli command.Cli, features features.Features) *cobra.Command {
	dockerClient := docker.NewClient(dockerCli)

	cmd := &cobra.Command{
		Use:              "mcp [OPTIONS]",
		Short:            "Manage MCP servers and clients",
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

			// Check the feature initialization error here for clearer error messages for the user
			if features.InitError() != nil {
				return features.InitError()
			}

			if os.Getenv("DOCKER_MCP_IN_CONTAINER") != "1" {
				if features.IsProfilesFeatureEnabled() {
					if isSubcommandOf(cmd, []string{"catalog-next", "catalog", "profile"}) {
						dao, err := db.New()
						if err != nil {
							return err
						}
						defer dao.Close()
						migrate.MigrateConfig(cmd.Context(), dockerClient, dao)
					}
				}

				runningInDockerCE, err := docker.RunningInDockerCE(ctx, dockerCli)
				if err != nil {
					return err
				}

				if !runningInDockerCE {
					return desktop.CheckFeatureIsEnabled(ctx, "enableDockerMCPToolkit", "Docker MCP Toolkit")
				}
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
	cmd.AddCommand(gatewayCommand(dockerClient, dockerCli, features))
	cmd.AddCommand(disableCommand(featureCommand(dockerCli, features)))
	cmd.AddCommand(disableCommand(versionCommand()))

	if features.IsProfilesFeatureEnabled() {
		cmd.AddCommand(workingSetCommand())
		cmd.AddCommand(catalogNextCommand())
		// Disable other commands when profiles feature is enabled
		cmd.AddCommand(disableCommand(catalogCommand(dockerCli)))
		cmd.AddCommand(disableCommand(configCommand(dockerClient)))
		cmd.AddCommand(disableCommand(policyCommand()))
		cmd.AddCommand(disableCommand(registryCommand()))
		cmd.AddCommand(disableCommand(secretCommand(dockerClient)))
		cmd.AddCommand(disableCommand(serverCommand(dockerClient, dockerCli)))
		cmd.AddCommand(disableCommand(toolsCommand(dockerClient, dockerCli)))
	} else {
		// When profiles feature is disabled, enable all commands normally
		cmd.AddCommand(catalogCommand(dockerCli))
		cmd.AddCommand(configCommand(dockerClient))
		cmd.AddCommand(policyCommand())
		cmd.AddCommand(registryCommand())
		cmd.AddCommand(secretCommand(dockerClient))
		cmd.AddCommand(serverCommand(dockerClient, dockerCli))
		cmd.AddCommand(toolsCommand(dockerClient, dockerCli))
	}
	cmd.AddCommand(clientCommand(dockerCli, cwd, features))
	cmd.AddCommand(oauthCommand())

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

func isSubcommandOf(cmd *cobra.Command, names []string) bool {
	if cmd == nil {
		return false
	}

	if slices.Contains(names, cmd.Name()) {
		return true
	}

	return isSubcommandOf(cmd.Parent(), names)
}
