package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clientcli "github.com/docker/mcp-gateway/cmd/docker-mcp/client"
	"github.com/docker/mcp-gateway/pkg/client"
	"github.com/docker/mcp-gateway/pkg/features"
)

func clientCommand(dockerCli command.Cli, cwd string, features features.Features) *cobra.Command {
	cfg := client.ReadConfig()
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("client (Supported: %s)", strings.Join(client.GetSupportedMCPClients(*cfg), ", ")),
		Short: "Manage MCP clients",
	}
	cmd.AddCommand(listClientCommand(cwd, *cfg))
	cmd.AddCommand(connectClientCommand(dockerCli, cwd, *cfg, features))
	cmd.AddCommand(disconnectClientCommand(cwd, *cfg))
	cmd.AddCommand(manualClientCommand(features))
	return cmd
}

func listClientCommand(cwd string, cfg client.Config) *cobra.Command {
	var opts struct {
		Global bool
		JSON   bool
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List client configurations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return clientcli.List(cmd.Context(), cwd, cfg, opts.Global, opts.JSON)
		},
	}
	flags := cmd.Flags()
	addGlobalFlag(flags, &opts.Global)
	flags.BoolVar(&opts.JSON, "json", false, "Print as JSON.")
	return cmd
}

func connectClientCommand(dockerCli command.Cli, cwd string, cfg client.Config, features features.Features) *cobra.Command {
	var opts struct {
		Global     bool
		Quiet      bool
		WorkingSet string
	}
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("connect [OPTIONS] <mcp-client>\n\nSupported clients: %s", strings.Join(client.GetSupportedMCPClients(cfg), " ")),
		Short: fmt.Sprintf("Connect the Docker MCP Toolkit to a client. Supported clients: %s", strings.Join(client.GetSupportedMCPClients(cfg), " ")),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return clientcli.Connect(cmd.Context(), dockerCli, cwd, cfg, args[0], opts.Global, opts.Quiet, opts.WorkingSet)
		},
	}
	flags := cmd.Flags()
	addGlobalFlag(flags, &opts.Global)
	addQuietFlag(flags, &opts.Quiet)
	if features.IsProfilesFeatureEnabled() {
		addWorkingSetFlag(flags, &opts.WorkingSet)
	}
	return cmd
}

func disconnectClientCommand(cwd string, cfg client.Config) *cobra.Command {
	var opts struct {
		Global bool
		Quiet  bool
	}
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("disconnect [OPTIONS] <mcp-client>\n\nSupported clients: %s", strings.Join(client.GetSupportedMCPClients(cfg), " ")),
		Short: fmt.Sprintf("Disconnect the Docker MCP Toolkit from a client. Supported clients: %s", strings.Join(client.GetSupportedMCPClients(cfg), " ")),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return clientcli.Disconnect(cmd.Context(), cwd, cfg, args[0], opts.Global, opts.Quiet)
		},
	}
	flags := cmd.Flags()
	addGlobalFlag(flags, &opts.Global)
	addQuietFlag(flags, &opts.Quiet)
	return cmd
}

func manualClientCommand(features features.Features) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manual-instructions",
		Short: "Display the manual instructions to connect the MCP client",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			printAsJSON, err := cmd.Flags().GetBool("json")
			if err != nil {
				return err
			}

			command := []string{"docker", "mcp", "gateway", "run"}
			if features.IsProfilesFeatureEnabled() {
				gordonProfile, err := client.ReadGordonProfile()
				if err != nil {
					return fmt.Errorf("failed to read gordon profile: %w", err)
				}
				if gordonProfile != "" {
					command = append(command, "--profile", gordonProfile)
				}
				fmt.Fprintf(os.Stderr, "Deprecation notice: This command is deprecated and only used for Gordon in Docker Desktop. Please use `docker mcp profile manual-instructions <profile-id>` instead.\n")
			}
			if printAsJSON {
				buf, err := json.Marshal(command)
				if err != nil {
					return err
				}
				_, _ = cmd.OutOrStdout().Write(buf)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), strings.Join(command, " "))
			}

			return nil
		},
		Hidden: true,
	}
	cmd.Flags().Bool("json", false, "Print as JSON.")
	return cmd
}

func addGlobalFlag(flags *pflag.FlagSet, p *bool) {
	flags.BoolVarP(p, "global", "g", false, "Change the system wide configuration or the clients setup in your current git repo.")
}

func addQuietFlag(flags *pflag.FlagSet, p *bool) {
	flags.BoolVarP(p, "quiet", "q", false, "Only display errors.")
}

func addWorkingSetFlag(flags *pflag.FlagSet, p *string) {
	flags.StringVarP(p, "profile", "p", "", "Profile to use for client connection.")
}
