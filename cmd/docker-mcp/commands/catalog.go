package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	catalogTypes "github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/yq"
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
	var mcpRegistry string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import <alias|url|file>",
		Short: "Import a catalog from URL or file",
		Long: `Import an MCP server catalog from a URL or local file. The catalog will be downloaded 
and stored locally for use with the MCP gateway.

When --mcp-registry flag is used, the argument must be an existing catalog name, and the
command will import servers from the MCP registry URL into that catalog.`,
		Args: cobra.ExactArgs(1),
		Example: `  # Import from URL
  docker mcp catalog import https://example.com/my-catalog.yaml
  
  # Import from local file
  docker mcp catalog import ./shared-catalog.yaml
  
  # Import from MCP registry URL into existing catalog
  docker mcp catalog import my-catalog --mcp-registry https://registry.example.com/server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If mcp-registry flag is provided, import to existing catalog
			if mcpRegistry != "" {
				if dryRun {
					return runMcpregistryImport(cmd.Context(), mcpRegistry, nil)
				}
				return importMCPRegistryToCatalog(cmd.Context(), args[0], mcpRegistry)
			}
			// Default behavior: import entire catalog
			return catalog.Import(cmd.Context(), args[0])
		},
	}
	cmd.Flags().StringVar(&mcpRegistry, "mcp-registry", "", "Import server from MCP registry URL into existing catalog")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show Imported Data but do not update the Catalog")
	return cmd
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
		Format catalog.Format
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all configured catalogs",
		Long:  `List all configured catalogs including Docker's official catalog and any locally managed catalogs.`,
		Args:  cobra.NoArgs,
		Example: `  # List all catalogs
  docker mcp catalog ls

  # List catalogs in JSON format
  docker mcp catalog ls --format=json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return catalog.Ls(cmd.Context(), opts.Format)
		},
	}
	flags := cmd.Flags()
	flags.Var(&opts.Format, "format", fmt.Sprintf("Output format. Supported: %s.", catalog.SupportedFormats()))
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

// importMCPRegistryToCatalog imports a server from an MCP registry URL into an existing catalog
func importMCPRegistryToCatalog(ctx context.Context, catalogName, mcpRegistryURL string) error {
	// Check if the catalog exists
	cfg, err := catalog.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read catalog config: %w", err)
	}

	_, exists := cfg.Catalogs[catalogName]
	if !exists {
		return fmt.Errorf("catalog '%s' does not exist", catalogName)
	}

	// Prevent users from modifying the Docker catalog
	if catalogName == catalog.DockerCatalogName {
		return fmt.Errorf("cannot import servers into catalog '%s' as it is managed by Docker", catalogName)
	}

	// Fetch server from MCP registry
	var servers []catalogTypes.Server
	if err := runMcpregistryImport(ctx, mcpRegistryURL, &servers); err != nil {
		return fmt.Errorf("failed to fetch server from MCP registry: %w", err)
	}

	if len(servers) == 0 {
		return fmt.Errorf("no servers found at MCP registry URL")
	}

	// For now, we'll import the first server (MCP registry URLs typically contain one server)
	server := servers[0]

	serverName := server.Name

	// Convert the server to JSON for injection into the catalog
	serverJSON, err := json.Marshal(server)
	if err != nil {
		return fmt.Errorf("failed to marshal server: %w", err)
	}

	// Read the current catalog content
	catalogContent, err := catalog.ReadCatalogFile(catalogName)
	if err != nil {
		return fmt.Errorf("failed to read catalog file: %w", err)
	}

	// Inject the server into the catalog using the same pattern as the add function
	updatedContent, err := injectServerIntoCatalog(catalogContent, serverName, serverJSON)
	if err != nil {
		return fmt.Errorf("failed to inject server into catalog: %w", err)
	}

	// Write the updated catalog back
	if err := catalog.WriteCatalogFile(catalogName, updatedContent); err != nil {
		return fmt.Errorf("failed to write updated catalog: %w", err)
	}

	fmt.Printf("Successfully imported server '%s' from MCP registry into catalog '%s'\n", serverName, catalogName)
	return nil
}

// injectServerIntoCatalog injects a server JSON into a catalog YAML using yq
func injectServerIntoCatalog(yamlData []byte, serverName string, serverJSON []byte) ([]byte, error) {
	query := fmt.Sprintf(`.registry."%s" = %s`, serverName, string(serverJSON))
	return yq.Evaluate(query, yamlData, yq.NewYamlDecoder(), yq.NewYamlEncoder())
}
