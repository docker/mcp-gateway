package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/pkg/client"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/pkg/sliceutil"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func workingSetCommand() *cobra.Command {
	cfg := client.ReadConfig()

	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles",
	}

	cmd.AddCommand(exportWorkingSetCommand())
	cmd.AddCommand(importWorkingSetCommand())
	cmd.AddCommand(showWorkingSetCommand())
	cmd.AddCommand(listWorkingSetsCommand())
	cmd.AddCommand(pushWorkingSetCommand())
	cmd.AddCommand(pullWorkingSetCommand())
	cmd.AddCommand(createWorkingSetCommand(cfg))
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

func createWorkingSetCommand(cfg *client.Config) *cobra.Command {
	var opts struct {
		ID      string
		Name    string
		Servers []string
		Connect []string
	}

	cmd := &cobra.Command{
		Use:   "create --name <name> [--id <id>] --server <ref1> --server <ref2> ... [--connect <client1> --connect <client2> ...]",
		Short: "Create a new profile of MCP servers",
		Long: `Create a new profile that groups multiple MCP servers together.
A profile allows you to organize and manage related servers as a single unit.
Profiles are decoupled from catalogs. Servers can be:
  - MCP Registry references (e.g. http://registry.modelcontextprotocol.io/v0/servers/312e45a4-2216-4b21-b9a8-0f1a51425073)
  - OCI image references with docker:// prefix (e.g., "docker://my-server:latest"). Images must be self-describing.
	- Catalog references with catalog:// prefix (e.g., "catalog://mcp/docker-mcp-catalog/github+obsidian").`,
		Example: `  # Create a profile with servers from a catalog
  docker mcp profile create --name dev-tools --server catalog://mcp/docker-mcp-catalog/github+obsidian

  # Create a profile with multiple servers (OCI references)
  docker mcp profile create --name my-profile --server docker://my-server:latest --server docker://my-other-server:latest

  # Create a profile with MCP Registry references
  docker mcp profile create --name my-profile --server http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

  # Connect to clients upon creation
  docker mcp profile create --name dev-tools --connect cursor`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			registryClient := registryapi.NewClient()
			ociService := oci.NewService()
			return workingset.Create(cmd.Context(), dao, registryClient, ociService, opts.ID, opts.Name, opts.Servers, opts.Connect)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Name, "name", "", "Name of the profile (required)")
	flags.StringVar(&opts.ID, "id", "", "ID of the profile (defaults to a slugified version of the name)")
	flags.StringArrayVar(&opts.Servers, "server", []string{}, "Server to include specified with a URI: https:// (MCP Registry reference) or docker:// (Docker Image reference) or catalog:// (Catalog reference). Can be specified multiple times.")
	flags.StringArrayVar(&opts.Connect, "connect", []string{}, fmt.Sprintf("Clients to connect to: mcp-client (can be specified multiple times). Supported clients: %s", supportedClientsList(*cfg)))
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
	var showClients bool

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
			return workingset.Show(cmd.Context(), dao, args[0], workingset.OutputFormat(format), showClients)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))
	flags.BoolVar(&showClients, "clients", false, "Include client information in output")
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

func listServersCommand() *cobra.Command {
	var opts struct {
		Filters []string
		Format  string
	}

	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List servers across profiles",
		Long: `List all servers grouped by profile.

Use --filter to search for servers matching a query (case-insensitive substring matching on server names).
Filters use key=value format (e.g., name=github, profile=my-dev-env).`,
		Example: `  # List all servers across all profiles
  docker mcp profile server ls

  # Filter servers by name
  docker mcp profile server ls --filter name=github

  # Show servers from a specific profile
  docker mcp profile server ls --filter profile=my-dev-env

  # Combine multiple filters (using short flag)
  docker mcp profile server ls -f name=slack -f profile=my-dev-env

  # Output in JSON format
  docker mcp profile server ls --format json`,
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

			return workingset.ListServers(cmd.Context(), dao, opts.Filters, workingset.OutputFormat(opts.Format))
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVarP(&opts.Filters, "filter", "f", []string{}, "Filter output (e.g., name=github, profile=my-dev-env)")
	flags.StringVar(&opts.Format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

	return cmd
}

func workingsetServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers in profiles",
	}

	cmd.AddCommand(listServersCommand())
	cmd.AddCommand(addServerCommand())
	cmd.AddCommand(removeServerCommand())
	cmd.AddCommand(updateServerCommand())

	return cmd
}

func addServerCommand() *cobra.Command {
	var servers []string

	cmd := &cobra.Command{
		Use:   "add <profile-id> [--server <ref1> --server <ref2> ...]",
		Short: "Add MCP servers to a profile",
		Long:  "Add MCP servers to a profile.",
		Example: `  # Add servers from a catalog
  docker mcp profile server add dev-tools --server catalog://mcp/docker-mcp-catalog/github+obsidian

  # Add servers with OCI references
  docker mcp profile server add my-profile --server docker://my-server:latest

  # Add servers with MCP Registry references
  docker mcp profile server add my-profile --server http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

  # Mix server references
  docker mcp profile server add dev-tools --server catalog://mcp/docker-mcp-catalog/github+obsidian --server docker://my-server:latest`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			registryClient := registryapi.NewClient()
			ociService := oci.NewService()
			return workingset.AddServers(cmd.Context(), dao, registryClient, ociService, args[0], servers)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&servers, "server", []string{}, "Server to include specified with a URI: https:// (MCP Registry reference) or docker:// (Docker Image reference) or catalog:// (Catalog reference). Can be specified multiple times.")

	return cmd
}

func removeServerCommand() *cobra.Command {
	var names []string
	var servers []string

	cmd := &cobra.Command{
		Use:     "remove <profile-id> [--name <name1> ...] [--server <uri1> ...]",
		Aliases: []string{"rm"},
		Short:   "Remove MCP servers from a profile",
		Long:    "Remove MCP servers from a profile by server name or server URI.",
		Example: `  # Remove by name
  docker mcp profile server remove dev-tools --name github --name slack

  # Remove by URI (same as used for add)
  docker mcp profile server remove dev-tools --server catalog://mcp/docker-mcp-catalog/github+slack

  # Remove by direct image reference
  docker mcp profile server remove dev-tools --server docker://mcp/github:latest

  # Mix multiple URIs
  docker mcp profile server remove dev-tools --server catalog://mcp/docker-mcp-catalog/github --server docker://custom-server:latest`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validation: can't specify both
			if len(names) > 0 && len(servers) > 0 {
				return fmt.Errorf("cannot specify both --name and --server flags")
			}
			if len(names) == 0 && len(servers) == 0 {
				return fmt.Errorf("must specify either --name or --server flag")
			}

			dao, err := db.New()
			if err != nil {
				return err
			}

			// If servers provided, resolve to names first
			if len(servers) > 0 {
				registryClient := registryapi.NewClient()
				ociService := oci.NewService()
				names, err = workingset.ResolveServerURIsToNames(cmd.Context(), dao, registryClient, ociService, servers)
				if err != nil {
					return fmt.Errorf("failed to resolve server URIs: %w\nHint: Use --name flag with server names from 'docker mcp profile show %s'", err, args[0])
				}
			}

			return workingset.RemoveServers(cmd.Context(), dao, args[0], names)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&names, "name", []string{}, "Server name to remove (can be specified multiple times)")
	flags.StringArrayVar(&servers, "server", []string{}, "Server URI to remove - same format as add command (can be specified multiple times)")

	return cmd
}

func updateServerCommand() *cobra.Command {
	var addServers []string
	var removeServers []string

	cmd := &cobra.Command{
		Use:   "update <profile-id> [--add <uri1> --add <uri2> ...] [--remove <uri1> --remove <uri2> ...]",
		Short: "Update servers in a profile (add and remove atomically)",
		Long:  "Atomically add and remove MCP servers in a single operation. Both operations are applied together or fail together.",
		Example: `  # Add and remove servers in one atomic operation
  docker mcp profile server update dev-tools --add catalog://mcp/docker-mcp-catalog/github --remove catalog://mcp/docker-mcp-catalog/slack

  # Add multiple servers while removing others
  docker mcp profile server update my-profile --add docker://server1:latest --add docker://server2:latest --remove docker://old-server:latest

  # Mix different URI types
  docker mcp profile server update dev-tools --add catalog://mcp/docker-mcp-catalog/github --add http://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860 --remove docker://old:latest`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(addServers) == 0 && len(removeServers) == 0 {
				return fmt.Errorf("must specify at least one --add or --remove flag")
			}

			dao, err := db.New()
			if err != nil {
				return err
			}
			registryClient := registryapi.NewClient()
			ociService := oci.NewService()
			return workingset.UpdateServers(cmd.Context(), dao, registryClient, ociService, args[0], addServers, removeServers)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&addServers, "add", []string{}, "Server URI to add: https:// (MCP Registry) or docker:// (Docker Image) or catalog:// (Catalog). Can be specified multiple times.")
	flags.StringArrayVar(&removeServers, "remove", []string{}, "Server URI to remove - same format as add. Can be specified multiple times.")

	return cmd
}

func supportedClientsList(cfg client.Config) string {
	// Gordon doesn't support profiles yet
	return strings.Join(sliceutil.Filter(client.GetSupportedMCPClients(cfg), func(c string) bool {
		return c != client.VendorGordon
	}), " ")
}
