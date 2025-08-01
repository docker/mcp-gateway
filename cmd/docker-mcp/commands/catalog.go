package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
)

func catalogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "catalog",
		Aliases: []string{"catalogs"},
		Short:   "Manage MCP server catalogs",
		Long:    `Manage MCP server catalogs for organizing and configuring custom MCP servers alongside Docker's official catalog.`,
	}
	cmd.AddCommand(bootstrapCatalogCommand())
	cmd.AddCommand(importCatalogCommand())
	cmd.AddCommand(exportCatalogCommand())
	cmd.AddCommand(lsCatalogCommand())
	cmd.AddCommand(rmCatalogCommand())
	cmd.AddCommand(updateCatalogCommand())
	cmd.AddCommand(showCatalogCommand())
	cmd.AddCommand(forkCatalogCommand())
	cmd.AddCommand(createCatalogCommand())
	cmd.AddCommand(initCatalogCommand())
	cmd.AddCommand(addCatalogCommand())
	cmd.AddCommand(resetCatalogCommand())
	return cmd
}

func importCatalogCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "import <alias|url|file>",
		Short: "Import a catalog from URL or file",
		Long: `Import an MCP server catalog from a URL or local file. The catalog will be downloaded 
and stored locally for use with the MCP gateway.`,
		Args: cobra.ExactArgs(1),
		Example: `  # Import from URL
  docker mcp catalog import https://example.com/my-catalog.yaml
  
  # Import from local file
  docker mcp catalog import ./shared-catalog.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return catalog.Import(cmd.Context(), args[0])
		},
	}
}

func exportCatalogCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "export <catalog-name> <file-path>",
		Short: "Export a configured catalog to a file",
		Long: `Export a user-managed catalog to a file. This command only works with catalogs
that have been imported or configured manually. The canonical Docker MCP catalog
cannot be exported as it is managed by Docker.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return catalog.Export(cmd.Context(), args[0], args[1])
		},
	}
}

func lsCatalogCommand() *cobra.Command {
	var opts struct {
		JSON bool
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all configured catalogs",
		Long:  `List all configured catalogs including Docker's official catalog and any locally managed catalogs.`,
		Args:  cobra.NoArgs,
		Example: `  # List all catalogs
  docker mcp catalog ls
  
  # List catalogs in JSON format
  docker mcp catalog ls --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return catalog.Ls(cmd.Context(), opts.JSON)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.JSON, "json", false, "Print as JSON.")
	return cmd
}

func rmCatalogCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a catalog",
		Long: `Remove a locally configured catalog. This will delete the catalog and all its server definitions.
The Docker official catalog cannot be removed.`,
		Args: cobra.ExactArgs(1),
		Example: `  # Remove a catalog
  docker mcp catalog rm old-servers`,
		RunE: func(_ *cobra.Command, args []string) error {
			return catalog.Rm(args[0])
		},
	}
}

func updateCatalogCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update [name]",
		Short: "Update catalog(s) from remote sources",
		Long: `Update one or more catalogs by re-downloading from their original sources.
If no name is provided, updates all catalogs that have remote sources.`,
		Args: cobra.MaximumNArgs(1),
		Example: `  # Update all catalogs
  docker mcp catalog update
  
  # Update specific catalog
  docker mcp catalog update team-servers`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return catalog.Update(cmd.Context(), args)
		},
	}
}

func showCatalogCommand() *cobra.Command {
	var opts struct {
		Format catalog.Format
	}
	cmd := &cobra.Command{
		Use:   "show [name]",
		Short: "Display catalog contents",
		Long: `Display the contents of a catalog including all server definitions and metadata.
If no name is provided, shows the Docker official catalog.`,
		Args: cobra.MaximumNArgs(1),
		Example: `  # Show Docker's official catalog
  docker mcp catalog show
  
  # Show a specific catalog in JSON format
  docker mcp catalog show my-catalog --format=json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := catalog.DockerCatalogName
			if len(args) > 0 {
				name = args[0]
			}

			return catalog.Show(cmd.Context(), name, opts.Format)
		},
	}
	flags := cmd.Flags()
	flags.Var(&opts.Format, "format", fmt.Sprintf("Supported: %s.", catalog.SupportedFormats()))
	return cmd
}

func forkCatalogCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "fork <src-catalog> <new-name>",
		Short: "Create a copy of an existing catalog",
		Long:  `Create a new catalog by copying all servers from an existing catalog. Useful for creating variations of existing catalogs.`,
		Args:  cobra.ExactArgs(2),
		Example: `  # Fork the Docker catalog to customize it
  docker mcp catalog fork docker-mcp my-custom-docker
  
  # Fork a team catalog for personal use
  docker mcp catalog fork team-servers my-servers`,
		RunE: func(_ *cobra.Command, args []string) error {
			return catalog.Fork(args[0], args[1])
		},
	}
}

func createCatalogCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new empty catalog",
		Long:  `Create a new empty catalog for organizing custom MCP servers. The catalog will be stored locally and can be populated using 'docker mcp catalog add'.`,
		Args:  cobra.ExactArgs(1),
		Example: `  # Create a new catalog for development servers
  docker mcp catalog create dev-servers
  
  # Create a catalog for production monitoring tools  
  docker mcp catalog create prod-monitoring`,
		RunE: func(_ *cobra.Command, args []string) error {
			return catalog.Create(args[0])
		},
	}
}

func initCatalogCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the catalog system",
		Long:  `Initialize the local catalog management system by creating the necessary configuration files and directories.`,
		Args:  cobra.NoArgs,
		Example: `  # Initialize catalog system
  docker mcp catalog init`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return catalog.Init(cmd.Context())
		},
	}
}

func addCatalogCommand() *cobra.Command {
	var opts struct {
		Force bool
	}
	cmd := &cobra.Command{
		Use:   "add <catalog> <server-name> <catalog-file>",
		Short: "Add a server to a catalog",
		Long: `Add an MCP server definition to an existing catalog by copying it from another catalog file.
The server definition includes all configuration, tools, and metadata.`,
		Args: cobra.ExactArgs(3),
		Example: `  # Add a server from another catalog file
  docker mcp catalog add my-catalog github-server ./github-catalog.yaml
  
  # Add with force to overwrite existing server
  docker mcp catalog add my-catalog slack-bot ./team-catalog.yaml --force`,
		RunE: func(_ *cobra.Command, args []string) error {
			parsedArgs := catalog.ParseAddArgs(args[0], args[1], args[2])
			if err := catalog.ValidateArgs(*parsedArgs); err != nil {
				return err
			}
			return catalog.Add(*parsedArgs, opts.Force)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.Force, "force", false, "Overwrite existing server in the catalog")
	return cmd
}

func resetCatalogCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "reset",
		Aliases: []string{"empty"},
		Short:   "Reset the catalog system",
		Long:    `Reset the local catalog management system by removing all user-managed catalogs and configuration. This does not affect Docker's official catalog.`,
		Args:    cobra.NoArgs,
		Example: `  # Reset all user catalogs
  docker mcp catalog reset`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return catalog.Reset(cmd.Context())
		},
	}
}
