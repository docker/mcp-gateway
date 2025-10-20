package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	workingset "github.com/docker/mcp-gateway/cmd/docker-mcp/working-set"
)

func workingSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "working-set",
		Aliases: []string{"ws"},
		Short:   "Manage MCP server working-sets",
		Long:    `Manage working-sets of MCP servers for organizing and grouping servers together.`,
	}
	cmd.AddCommand(createWorkingSetCommand())
	cmd.AddCommand(listWorkingSetCommand())
	cmd.AddCommand(showWorkingSetCommand())
	return cmd
}

func createWorkingSetCommand() *cobra.Command {
	var opts struct {
		Name        string
		Description string
		Servers     []string
	}
	cmd := &cobra.Command{
		Use:   "create --name <name> [--description <description>] --server <server1> --server <server2> ...",
		Short: "Create a new working-set of MCP servers",
		Long: `Create a new working-set that groups multiple MCP servers together.
A working-set allows you to organize and manage related servers as a single unit.`,
		Example: `  # Create a working-set with multiple servers
  docker mcp working-set create --name dev-tools --description "Development tools" --server github --server slack

  # Create a working-set with a single server
  docker mcp working-set create --name docker-only --server docker`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return workingset.Create(opts.Name, opts.Description, opts.Servers)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Name, "name", "", "Name of the working-set (required)")
	flags.StringVar(&opts.Description, "description", "", "Description of the working-set")
	flags.StringArrayVar(&opts.Servers, "server", []string{}, "Server to include in the working-set (can be specified multiple times)")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server")

	return cmd
}

func listWorkingSetCommand() *cobra.Command {
	var opts struct {
		Format workingset.Format
	}
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List all working-sets",
		Long:    `List all configured working-sets and their associated servers.`,
		Example: `  # List all working-sets in human-readable format
  docker mcp working-set ls

  # List working-sets in JSON format
  docker mcp working-set ls --format json

  # List working-sets in YAML format
  docker mcp working-set ls --format yaml`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return workingset.List(opts.Format)
		},
	}
	flags := cmd.Flags()
	flags.Var(&opts.Format, "format", fmt.Sprintf("Output format. Supported: %s.", workingset.SupportedFormats()))
	return cmd
}

func showWorkingSetCommand() *cobra.Command {
	var opts struct {
		Format workingset.Format
	}
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Display working-set details",
		Long:  `Display the details of a specific working-set including all its servers.`,
		Example: `  # Show a working-set in human-readable format
  docker mcp working-set show my-dev-tools

  # Show a working-set in JSON format
  docker mcp working-set show my-dev-tools --format json

  # Show a working-set in YAML format
  docker mcp working-set show my-dev-tools --format yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return workingset.Show(args[0], opts.Format)
		},
	}
	flags := cmd.Flags()
	flags.Var(&opts.Format, "format", fmt.Sprintf("Output format. Supported: %s.", workingset.SupportedFormats()))
	return cmd
}
