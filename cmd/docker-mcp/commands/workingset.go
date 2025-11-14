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
		Use:   "profile",
		Short: "Manage profiles",
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
	cmd.AddCommand(toolsWorkingSetCommand())
	return cmd
}

func configWorkingSetCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)
	getAll := false
	var set []string
	var get []string
	var del []string

	cmd := &cobra.Command{
		Use:   "config <profile-id> [--set <config-arg1> <config-arg2> ...] [--get <config-key1> <config-key2> ...] [--del <config-arg1> <config-arg2> ...]",
		Short: "Update the configuration of a profile",
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

func toolsWorkingSetCommand() *cobra.Command {
	var enable []string
	var disable []string
	var enableAll []string
	var disableAll []string

	cmd := &cobra.Command{
		Use:   "tools <profile-id> [--enable <tool> ...] [--disable <tool> ...] [--enable-all <server> ...] [--disable-all <server> ...]",
		Short: "Manage tool allowlist for servers in a profile",
		Long: `Manage the tool allowlist for servers in a profile.
Tools are specified using dot notation: <serverName>.<toolName>

Use --enable to enable specific tools for a server (can be specified multiple times).
Use --disable to disable specific tools for a server (can be specified multiple times).
Use --enable-all to enable all tools for a server (can be specified multiple times).
Use --disable-all to disable all tools for a server (can be specified multiple times).

To view enabled tools, use: docker mcp profile show <profile-id>`,
		Example: `  # Enable specific tools for a server
  docker mcp profile tools my-profile --enable github.create_issue --enable github.list_repos

  # Disable specific tools for a server
  docker mcp profile tools my-profile --disable github.create_issue --disable github.search_code

  # Enable and disable in one command
  docker mcp profile tools my-profile --enable github.create_issue --disable github.search_code

  # Enable all tools for a server
  docker mcp profile tools my-profile --enable-all github

  # Disable all tools for a server
  docker mcp profile tools my-profile --disable-all github

  # View all enabled tools in the profile
  docker mcp profile show my-profile`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.UpdateTools(cmd.Context(), dao, args[0], enable, disable, enableAll, disableAll)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&enable, "enable", []string{}, "Enable specific tools: <serverName>.<toolName> (repeatable)")
	flags.StringArrayVar(&disable, "disable", []string{}, "Disable specific tools: <serverName>.<toolName> (repeatable)")
	flags.StringArrayVar(&enableAll, "enable-all", []string{}, "Enable all tools for a server: <serverName> (repeatable)")
	flags.StringArrayVar(&disableAll, "disable-all", []string{}, "Disable all tools for a server: <serverName> (repeatable)")

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
		Short: "Create a new profile of MCP servers",
		Long: `Create a new profile that groups multiple MCP servers together.
A profile allows you to organize and manage related servers as a single unit.
Profiles are decoupled from catalogs. Servers can be:
  - MCP Registry references (e.g. http://registry.modelcontextprotocol.io/v0/servers/312e45a4-2216-4b21-b9a8-0f1a51425073)
  - OCI image references with docker:// prefix (e.g., "docker://mcp/github:latest")`,
		Example: `  # Create a profile with multiple servers (OCI references)
  docker mcp profile create --name dev-tools --server docker://mcp/github:latest --server docker://mcp/slack:latest

  # Create a profile with MCP Registry references
  docker mcp profile create --name registry-servers --server http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

  # Mix MCP Registry references and OCI references
  docker mcp profile create --name mixed --server http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860 --server docker://mcp/github:latest`,
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
	flags.StringVar(&opts.Name, "name", "", "Name of the profile (required)")
	flags.StringVar(&opts.ID, "id", "", "ID of the profile (defaults to a slugified version of the name)")
	flags.StringArrayVar(&opts.Servers, "server", []string{}, "Server to include: catalog name or OCI reference with docker:// prefix (can be specified multiple times)")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func listWorkingSetsCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List profiles",
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
		Use:   "show <profile-id>",
		Short: "Show profile",
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
		Short: "Pull profile from OCI registry",
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
		Use:   "push <profile-id> <oci-reference>",
		Short: "Push profile to OCI registry",
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
		Use:   "export <profile-id> <output-file>",
		Short: "Export profile to file",
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
		Short: "Import profile from file",
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
		Use:     "remove <profile-id>",
		Aliases: []string{"rm"},
		Short:   "Remove a profile",
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
		Short: "List servers across profiles",
		Long: `List all servers grouped by profile.

Use --filter to search for servers matching a query (case-insensitive substring matching on image names or source URLs).
Use --profile to show servers only from a specific profile.`,
		Example: `  # List all servers across all profiles
  docker mcp profile servers

  # Filter servers by name
  docker mcp profile servers --filter github

  # Show servers from a specific profile
  docker mcp profile servers --profile dev-tools

  # Combine filter and profile
  docker mcp profile servers --profile dev-tools --filter slack

  # Output in JSON format
  docker mcp profile servers --format json`,
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
	flags.StringVarP(&opts.WorkingSetID, "profile", "p", "", "Show servers only from specified profile")
	flags.StringVar(&opts.Filter, "filter", "", "Filter servers by image name or source URL")
	flags.StringVar(&opts.Format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

	return cmd
}

func workingsetServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers in profiles",
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
		Use:   "add <profile-id> [--server <ref1> --server <ref2> ...] [--catalog <oci-reference> --catalog-server <server1> --catalog-server <server2> ...]",
		Short: "Add MCP servers to a profile",
		Long:  "Add MCP servers to a profile.",
		Example: ` # Add servers with OCI references
  docker mcp profile server add dev-tools --server docker://mcp/github:latest --server docker://mcp/slack:latest

  # Add servers with MCP Registry references
  docker mcp profile server add dev-tools --server http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

  # Mix MCP Registry references and OCI references
  docker mcp profile server add dev-tools --server http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860 --server docker://mcp/github:latest

  # Add servers from a catalog
  docker mcp profile server add dev-tools --catalog my-catalog --catalog-server github --catalog-server slack

  # Mix catalog servers with direct server references
  docker mcp profile server add dev-tools --catalog my-catalog --catalog-server github --server docker://mcp/slack:latest`,
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
		Use:     "remove <profile-id> --name <name1> --name <name2> ...",
		Aliases: []string{"rm"},
		Short:   "Remove MCP servers from a profile",
		Long:    "Remove MCP servers from a profile by server name.",
		Example: ` # Remove servers by name
  docker mcp profile server remove dev-tools --name github --name slack

  # Remove a single server
  docker mcp profile server remove dev-tools --name github`,
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
