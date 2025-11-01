package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func workingSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workingset",
		Short: "Manage working sets",
	}

	cmd.AddCommand(exportWorkingSetCommand())
	cmd.AddCommand(importWorkingSetCommand())
	cmd.AddCommand(showWorkingSetCommand())
	cmd.AddCommand(listWorkingSetsCommand())
	cmd.AddCommand(serversCommand())
	cmd.AddCommand(pushWorkingSetCommand())
	cmd.AddCommand(pullWorkingSetCommand())
	cmd.AddCommand(createWorkingSetCommand())
	cmd.AddCommand(removeWorkingSetCommand())
	return cmd
}

func createWorkingSetCommand() *cobra.Command {
	var opts struct {
		ID      string
		Name    string
		Servers []string
	}

	cmd := &cobra.Command{
		Use:   "create --name <name> [--id <id>] --server <ref1> --server <ref2> ...",
		Short: "Create a new working set of MCP servers",
		Long: `Create a new working set that groups multiple MCP servers together.
A working set allows you to organize and manage related servers as a single unit.
Working sets are decoupled from catalogs. Servers can be:
  - MCP Registry references (e.g. http://registry.modelcontextprotocol.io/v0/servers/312e45a4-2216-4b21-b9a8-0f1a51425073)
  - OCI image references with docker:// prefix (e.g., "docker://mcp/github:latest")`,
		Example: `  # Create a working-set with multiple servers (OCI references)
  docker mcp working-set create --name dev-tools --server docker://mcp/github:latest --server docker://mcp/slack:latest

  # Create a working-set with MCP Registry references
  docker mcp working-set create --name registry-servers --server http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

  # Mix MCP Registry references and OCI references
  docker mcp working-set create --name mixed --server http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860 --server docker://mcp/github:latest`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Create(cmd.Context(), dao, opts.ID, opts.Name, opts.Servers)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Name, "name", "", "Name of the working set (required)")
	flags.StringVar(&opts.ID, "id", "", "ID of the working set (defaults to a slugified version of the name)")
	flags.StringArrayVar(&opts.Servers, "server", []string{}, "Server to include: catalog name or OCI reference with docker:// prefix (can be specified multiple times)")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func listWorkingSetsCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List working sets",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			supported := slices.Contains(workingset.SupportedFormats(), format)
			if !supported {
				return fmt.Errorf("unsupported format: %s", format)
			}
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.List(cmd.Context(), dao, workingset.OutputFormat(format))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

	return cmd
}

func showWorkingSetCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)

	cmd := &cobra.Command{
		Use:   "show <working-set-id>",
		Short: "Show working set",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			supported := slices.Contains(workingset.SupportedFormats(), format)
			if !supported {
				return fmt.Errorf("unsupported format: %s", format)
			}
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Show(cmd.Context(), dao, args[0], workingset.OutputFormat(format))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

	return cmd
}

func pullWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <oci-reference>",
		Short: "Pull working set from OCI registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Pull(cmd.Context(), dao, args[0])
		},
	}
}

func pushWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "push <working-set-id> <oci-reference>",
		Short: "Push working set to OCI registry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Push(cmd.Context(), dao, args[0], args[1])
		},
	}
}

func exportWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "export <working-set-id> <output-file>",
		Short: "Export working set to file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Export(cmd.Context(), dao, args[0], args[1])
		},
	}
}

func importWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "import <input-file>",
		Short: "Import working set from file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Import(cmd.Context(), dao, args[0])
		},
	}
}

func removeWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <working-set-id>",
		Aliases: []string{"rm"},
		Short:   "Remove a working set",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Remove(cmd.Context(), dao, args[0])
		},
	}
}

func serversCommand() *cobra.Command {
	var opts struct {
		WorkingSetID string
		Filter       string
		Format       string
	}

	cmd := &cobra.Command{
		Use:   "servers",
		Short: "List servers across working sets",
		Long: `List all servers grouped by working set.

Use --filter to search for servers matching a query (case-insensitive substring matching on image names or source URLs).
Use --workingset to show servers only from a specific working set.`,
		Example: `  # List all servers across all working sets
  docker mcp workingset servers

  # Filter servers by name
  docker mcp workingset servers --filter github

  # Show servers from a specific working set
  docker mcp workingset servers --workingset my-dev-env

  # Combine filter and working set
  docker mcp workingset servers --workingset my-dev-env --filter slack

  # Output in JSON format
  docker mcp workingset servers --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			supported := slices.Contains(workingset.SupportedFormats(), opts.Format)
			if !supported {
				return fmt.Errorf("unsupported format: %s", opts.Format)
			}

			dao, err := db.New()
			if err != nil {
				return err
			}

			return workingset.Servers(cmd.Context(), dao, opts.Filter, opts.WorkingSetID, workingset.OutputFormat(opts.Format))
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.WorkingSetID, "workingset", "w", "", "Show servers only from specified working set")
	flags.StringVar(&opts.Filter, "filter", "", "Filter servers by image name or source URL")
	flags.StringVar(&opts.Format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

	return cmd
}
