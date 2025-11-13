package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
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
	cmd.AddCommand(workingsetServerCommand())
	cmd.AddCommand(configWorkingSetCommand())
	return cmd
}

func configWorkingSetCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)
	getAll := false
	var set []string
	var get []string
	var del []string

	cmd := &cobra.Command{
		Use:   "config <working-set-id> [--set <config-arg1> <config-arg2> ...] [--get <config-key1> <config-key2> ...] [--del <config-arg1> <config-arg2> ...]",
		Short: "Update the configuration of a working set",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			supported := slices.Contains(workingset.SupportedFormats(), format)
			if !supported {
				return fmt.Errorf("unsupported format: %s", format)
			}
			dao, err := db.New()
			if err != nil {
				return err
			}
			ociService := oci.NewService()
			return workingset.UpdateConfig(cmd.Context(), dao, ociService, args[0], set, get, del, getAll, workingset.OutputFormat(format))
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&set, "set", []string{}, "Set configuration values: <key>=<value> (can be specified multiple times)")
	flags.StringArrayVar(&get, "get", []string{}, "Get configuration values: <key> (can be specified multiple times)")
	flags.StringArrayVar(&del, "del", []string{}, "Delete configuration values: <key> (can be specified multiple times)")
	flags.BoolVar(&getAll, "get-all", false, "Get all configuration values")
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

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
			registryClient := registryapi.NewClient()
			ociService := oci.NewService()
			return workingset.Create(cmd.Context(), dao, registryClient, ociService, opts.ID, opts.Name, opts.Servers)
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
			ociService := oci.NewService()
			return workingset.Pull(cmd.Context(), dao, ociService, args[0])
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
			ociService := oci.NewService()
			return workingset.Import(cmd.Context(), dao, ociService, args[0])
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

func workingsetServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers in working sets",
	}

	cmd.AddCommand(addServerCommand())
	cmd.AddCommand(removeServerCommand())

	return cmd
}

func addServerCommand() *cobra.Command {
	var servers []string
	var catalog string
	var catalogServers []string

	cmd := &cobra.Command{
		Use:   "add <working-set-id> [--server <ref1> --server <ref2> ...] [--catalog <oci-reference> --catalog-server <server1> --catalog-server <server2> ...]",
		Short: "Add MCP servers to a working set",
		Long:  "Add MCP servers to a working set.",
		Example: ` # Add servers with OCI references
  docker mcp workingset server add my-working-set --server docker://mcp/github:latest --server docker://mcp/slack:latest

  # Add servers with MCP Registry references
  docker mcp workingset server add my-working-set --server http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

  # Mix MCP Registry references and OCI references
  docker mcp workingset server add my-working-set --server http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860 --server docker://mcp/github:latest

  # Add servers from a catalog
  docker mcp workingset server add my-working-set --catalog my-catalog --catalog-server github --catalog-server slack

  # Mix catalog servers with direct server references
  docker mcp workingset server add my-working-set --catalog my-catalog --catalog-server github --server docker://mcp/slack:latest`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			registryClient := registryapi.NewClient()
			ociService := oci.NewService()
			return workingset.AddServers(cmd.Context(), dao, registryClient, ociService, args[0], servers, catalog, catalogServers)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&servers, "server", []string{}, "Server to include: MCP Registry reference or OCI reference with docker:// prefix (can be specified multiple times)")
	flags.StringVar(&catalog, "catalog", "", "Catalog to add servers from (optional)")
	flags.StringArrayVar(&catalogServers, "catalog-server", []string{}, "Server names from the catalog to add (can be specified multiple times, requires --catalog)")

	return cmd
}

func removeServerCommand() *cobra.Command {
	var names []string

	cmd := &cobra.Command{
		Use:     "remove <working-set-id> --name <name1> --name <name2> ...",
		Aliases: []string{"rm"},
		Short:   "Remove MCP servers from a working set",
		Long:    "Remove MCP servers from a working set by server name.",
		Example: ` # Remove servers by name
  docker mcp workingset server remove my-working-set --name github --name slack

  # Remove a single server
  docker mcp workingset server remove my-working-set --name github`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.RemoveServers(cmd.Context(), dao, args[0], names)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&names, "name", []string{}, "Server name to remove (can be specified multiple times)")

	return cmd
}
