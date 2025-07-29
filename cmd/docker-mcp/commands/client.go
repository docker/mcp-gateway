package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/pflag"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/client"
)

func clientCommand(cwd string) *cobra.Command {
	cfg := client.ReadConfig()
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("client (Supported: %s)", strings.Join(client.GetSupportedMCPClients(*cfg), ", ")),
		Short: "Manage MCP clients",
	}
	cmd.AddCommand(listClientCommand(cwd, *cfg))
	cmd.AddCommand(connectClientCommand(cwd, *cfg))
	cmd.AddCommand(disconnectClientCommand(cwd, *cfg))
	cmd.AddCommand(manualClientCommand())
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
			return client.List(cmd.Context(), cwd, cfg, opts.Global, opts.JSON)
		},
	}
	flags := cmd.Flags()
	addGlobalFlag(flags, &opts.Global)
	flags.BoolVar(&opts.JSON, "json", false, "Print as JSON.")
	return cmd
}

func connectClientCommand(cwd string, cfg client.Config) *cobra.Command {
	var opts struct {
		Global bool
		Quiet  bool
	}
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("connect [OPTIONS] <mcp-client>\n\nSupported clients: %s", strings.Join(client.GetSupportedMCPClients(cfg), " ")),
		Short: fmt.Sprintf("Connect the Docker MCP Toolkit to a client. Supported clients: %s", strings.Join(client.GetSupportedMCPClients(cfg), " ")),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.Connect(cmd.Context(), cwd, cfg, args[0], opts.Global, opts.Quiet)
		},
	}
	flags := cmd.Flags()
	addGlobalFlag(flags, &opts.Global)
	addQuietFlag(flags, &opts.Quiet)
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
			return client.Disconnect(cmd.Context(), cwd, cfg, args[0], opts.Global, opts.Quiet)
		},
	}
	flags := cmd.Flags()
	addGlobalFlag(flags, &opts.Global)
	addQuietFlag(flags, &opts.Quiet)
	return cmd
}

func manualClientCommand() *cobra.Command {
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
