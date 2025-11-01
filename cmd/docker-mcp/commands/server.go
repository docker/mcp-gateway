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
	"github.com/docker/mcp-gateway/pkg/terminal"
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
				// Format: $ docker mcp server ls
				// MCP Servers (7 enabled)
				//
				// NAME SECRETS CONFIG DESCRIPTION
				// atlassian ✓ done ✓ done Confluence and Jira tools

				enabledCount := len(list)
				fmt.Fprintf(cmd.OutOrStdout(), "\nMCP Servers (%d enabled)\n\n", enabledCount)

				// Calculate column widths based on terminal size
				termWidth := terminal.GetWidthFrom(cmd.OutOrStdout())
				colWidths := calculateColumnWidths(termWidth)

				// Calculate total table width (sum of columns + spaces between columns)
				totalWidth := colWidths.name + colWidths.oauth + colWidths.secrets + colWidths.config + colWidths.description + 4 // 4 spaces between columns

				// Print table headers
				fmt.Fprintf(cmd.OutOrStdout(), "%-*s %-*s %-*s %-*s %-*s\n",
					colWidths.name, "NAME",
					colWidths.oauth, "OAUTH",
					colWidths.secrets, "SECRETS",
					colWidths.config, "CONFIG",
					colWidths.description, "DESCRIPTION")
				fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", totalWidth))

				// Print entries
				for _, entry := range list {
					// Determine secrets, config, and OAuth display strings
					secretsText := entry.Secrets.DisplayString()
					configText := entry.Config.DisplayString()
					oauthText := entry.OAuth.DisplayString()

					// Truncate description to fit within the available column width
					description := truncateString(entry.Description, colWidths.description)

					fmt.Fprintf(cmd.OutOrStdout(), "%-*s %-*s %-*s %-*s %-*s\n",
						colWidths.name, truncateString(entry.Name, colWidths.name),
						colWidths.oauth, oauthText,
						colWidths.secrets, secretsText,
						colWidths.config, configText,
						colWidths.description, description)
				}

				if hints.Enabled(dockerCli) {
					fmt.Fprintln(cmd.OutOrStdout(), "")
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
				return oci.InspectArtifact[oci.Catalog](arg, oci.MCPServerArtifactType)
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
	var templateName string
	initCommand := &cobra.Command{
		Use:   "init <directory>",
		Short: "Initialize a new MCP server project",
		Long:  "Initialize a new MCP server project in the specified directory with boilerplate code, Dockerfile, and compose.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if err := server.Init(cmd.Context(), dir, language, templateName); err != nil {
				return err
			}
			serverName := filepath.Base(dir)
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully initialized MCP server project in %s (template: %s)\n", dir, templateName)
			fmt.Fprintf(cmd.OutOrStdout(), "Next steps:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  cd %s\n", dir)
			fmt.Fprintf(cmd.OutOrStdout(), "  docker build -t %s:latest .\n", serverName)
			fmt.Fprintf(cmd.OutOrStdout(), "  docker compose up\n")
			return nil
		},
	}
	initCommand.Flags().StringVar(&language, "language", "go", "Programming language for the server (currently only 'go' is supported)")
	initCommand.Flags().StringVar(&templateName, "template", "basic", "Template to use (basic, chatgpt-app-basic)")
	_ = initCommand.MarkFlagRequired("template")
	cmd.AddCommand(initCommand)

	return cmd
}

type columnWidths struct {
	name        int
	oauth       int
	secrets     int
	config      int
	description int
}

func calculateColumnWidths(termWidth int) columnWidths {
	// Minimum widths for each column
	minWidths := columnWidths{
		name:        15,
		oauth:       10,
		secrets:     10,
		config:      10,
		description: 20,
	}

	// Calculate minimum total width needed
	minTotal := minWidths.name + minWidths.oauth + minWidths.secrets + minWidths.config + minWidths.description + 4 // 4 spaces

	// If terminal is too narrow, use minimum widths
	if termWidth < minTotal+20 {
		return minWidths
	}

	// Available space after minimums and spacing
	available := termWidth - minTotal

	// Allocate extra space: 50% to description, 25% to name, 25% split between oauth/secrets/config
	result := columnWidths{
		name:        minWidths.name + available/4,
		oauth:       minWidths.oauth + available/12,
		secrets:     minWidths.secrets + available/12,
		config:      minWidths.config + available/12,
		description: minWidths.description + available/2,
	}

	return result
}

func truncateString(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if len(s) <= maxWidth {
		return s
	}
	if maxWidth > 3 {
		return s[:maxWidth-3] + "..."
	}
	return s[:maxWidth]
}
