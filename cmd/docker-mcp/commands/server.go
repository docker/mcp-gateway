package commands

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/hints"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/server"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/oci"
)

func serverCommand(docker docker.Client, dockerCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers",
	}

	var outputJSON bool
	lsCommand := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List enabled servers",
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
				if hints.Enabled(dockerCli) {
					hints.TipCyan.Fprint(cmd.OutOrStdout(), "Tip: To use these servers, connect to a client (IE: claude/cursor) with ")
					hints.TipCyanBoldItalic.Fprintln(cmd.OutOrStdout(), "docker mcp client connect <client-name>")
					fmt.Fprintln(cmd.OutOrStdout(), "")
				}
			}

			return nil
		},
	}
	lsCommand.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")
	cmd.AddCommand(lsCommand)

	cmd.AddCommand(&cobra.Command{
		Use:     "enable",
		Aliases: []string{"add"},
		Short:   "Enable a server or multiple servers",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpOAuthDcrEnabled := isMcpOAuthDcrFeatureEnabled(dockerCli)
			return server.Enable(cmd.Context(), docker, dockerCli, args, mcpOAuthDcrEnabled)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "disable",
		Aliases: []string{"remove", "rm"},
		Short:   "Disable a server or multiple servers",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpOAuthDcrEnabled := isMcpOAuthDcrFeatureEnabled(dockerCli)
			return server.Disable(cmd.Context(), docker, dockerCli, args, mcpOAuthDcrEnabled)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "inspect",
		Short: "Get information about a server or inspect an OCI artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arg := args[0]

			// Check if the argument looks like an OCI reference
			// OCI refs typically contain a registry/repository pattern with optional tag or digest
			if strings.Contains(arg, "/") && (strings.Contains(arg, ":") || strings.Contains(arg, "@")) {
				// Use OCI inspect for OCI references
				return oci.InspectArtifact(arg)
			}

			// Use regular server inspect for server names
			info, err := server.Inspect(cmd.Context(), docker, arg)
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

	var language string
	initCommand := &cobra.Command{
		Use:   "init <directory>",
		Short: "Initialize a new MCP server project",
		Long:  "Initialize a new MCP server project in the specified directory with boilerplate code, Dockerfile, and compose.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if err := server.Init(cmd.Context(), dir, language); err != nil {
				return err
			}
			serverName := filepath.Base(dir)
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully initialized MCP server project in %s\n", dir)
			fmt.Fprintf(cmd.OutOrStdout(), "Next steps:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  cd %s\n", dir)
			fmt.Fprintf(cmd.OutOrStdout(), "  docker build -t %s:latest .\n", serverName)
			fmt.Fprintf(cmd.OutOrStdout(), "  docker compose up\n")
			return nil
		},
	}
	initCommand.Flags().StringVar(&language, "language", "go", "Programming language for the server (currently only 'go' is supported)")
	cmd.AddCommand(initCommand)

	return cmd
}
