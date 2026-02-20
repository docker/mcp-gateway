package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	catalognext "github.com/docker/mcp-gateway/pkg/catalog_next"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func catalogNextCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "catalog",
		Aliases: []string{"catalogs", "catalog-next"},
		Short:   "Manage MCP server OCI catalogs",
	}

	cmd.AddCommand(createCatalogNextCommand())
	cmd.AddCommand(showCatalogNextCommand())
	cmd.AddCommand(listCatalogNextCommand())
	cmd.AddCommand(removeCatalogNextCommand())
	cmd.AddCommand(pushCatalogNextCommand())
	cmd.AddCommand(pullCatalogNextCommand())
	cmd.AddCommand(tagCatalogNextCommand())
	cmd.AddCommand(catalogNextServerCommand())

	return cmd
}

func createCatalogNextCommand() *cobra.Command {
	var opts struct {
		Title                 string
		FromWorkingSet        string
		FromLegacyCatalog     string
		FromCommunityRegistry string
		Servers               []string
		IncludePyPI           bool
	}

	cmd := &cobra.Command{
		Use:   "create <oci-reference> [--server <ref1> --server <ref2> ...] [--from-profile <profile-id>] [--from-legacy-catalog <url>] [--from-community-registry <hostname>] [--title <title>]",
		Short: "Create a new catalog from a profile, legacy catalog, or community registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceCount := 0
			if opts.FromWorkingSet != "" {
				sourceCount++
			}
			if opts.FromLegacyCatalog != "" {
				sourceCount++
			}
			if opts.FromCommunityRegistry != "" {
				sourceCount++
			}
			if sourceCount > 1 {
				return fmt.Errorf("only one of --from-profile, --from-legacy-catalog, or --from-community-registry can be specified")
			}

			if opts.IncludePyPI && opts.FromCommunityRegistry == "" {
				return fmt.Errorf("--include-pypi can only be used when creating a catalog from a community registry")
			}

			dao, err := db.New()
			if err != nil {
				return err
			}
			registryClient := registryapi.NewClient()
			ociService := oci.NewService()
			return catalognext.Create(cmd.Context(), dao, registryClient, ociService, args[0], opts.Servers, opts.FromWorkingSet, opts.FromLegacyCatalog, opts.FromCommunityRegistry, opts.Title, opts.IncludePyPI)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&opts.Servers, "server", []string{}, "Server to include specified with a URI: https:// (MCP Registry reference) or docker:// (Docker Image reference) or catalog:// (Catalog reference) or file:// (Local file path). Can be specified multiple times.")
	flags.StringVar(&opts.FromWorkingSet, "from-profile", "", "Profile ID to create the catalog from")
	flags.StringVar(&opts.FromLegacyCatalog, "from-legacy-catalog", "", "Legacy catalog URL to create the catalog from")
	flags.StringVar(&opts.FromCommunityRegistry, "from-community-registry", "", "Community registry hostname to fetch servers from (e.g. registry.modelcontextprotocol.io)")
	flags.StringVar(&opts.Title, "title", "", "Title of the catalog")

	flags.BoolVar(&opts.IncludePyPI, "include-pypi", false, "Include PyPI servers when creating a catalog from a community registry")
	cmd.Flags().MarkHidden("include-pypi") //nolint:errcheck

	return cmd
}

func tagCatalogNextCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tag SOURCE_IMAGE[:TAG] TARGET_IMAGE[:TAG]",
		Short: "Create a tagged copy of a catalog",
		Long: `Create a new catalog by tagging an existing catalog with a new name or version.
This creates a copy of the source catalog with a new reference, similar to Docker image tagging.`,
		Args: cobra.ExactArgs(2),
		Example: `  # Tag a catalog with a new version
  docker mcp catalog tag mcp/my-catalog:v1 mcp/my-catalog:v2

  # Create a tagged copy with a different name
  docker mcp catalog tag mcp/team-catalog:latest mcp/prod-catalog:v1.0

  # Tag without explicit version (uses latest)
  docker mcp catalog tag mcp/my-catalog mcp/my-catalog:backup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return catalognext.Tag(cmd.Context(), dao, args[0], args[1])
		},
	}
}

func showCatalogNextCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)
	pullOption := string(catalognext.PullOptionNever)
	var noTools bool
	var yqExpr string

	cmd := &cobra.Command{
		Use:   "show <oci-reference> [--pull <pull-option>]",
		Short: "Show a catalog",
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

			if noTools {
				if yqExpr != "" {
					return fmt.Errorf("cannot use --no-tools and --yq together")
				}
				yqExpr = "del(.servers[].tools, .servers[].snapshot.server.tools)"
			}

			ociService := oci.NewService()
			return catalognext.Show(cmd.Context(), dao, ociService, args[0], workingset.OutputFormat(format), pullOption, yqExpr)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))
	flags.StringVar(&pullOption, "pull", string(catalognext.PullOptionNever), fmt.Sprintf("Supported: %s, or duration (e.g. '1h', '1d'). Duration represents time since last update.", strings.Join(catalognext.SupportedPullOptions(), ", ")))
	flags.BoolVar(&noTools, "no-tools", false, "Exclude tools from output (deprecated, use --yq instead)")
	flags.StringVar(&yqExpr, "yq", "", "YQ expression to apply to the output")
	return cmd
}

func listCatalogNextCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List catalogs",
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
			return catalognext.List(cmd.Context(), dao, workingset.OutputFormat(format))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

	return cmd
}

func removeCatalogNextCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <oci-reference>",
		Aliases: []string{"rm"},
		Short:   "Remove a catalog",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return catalognext.Remove(cmd.Context(), dao, args[0])
		},
	}
}

func pushCatalogNextCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "push <oci-reference>",
		Short: "Push a catalog to an OCI registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return catalognext.Push(cmd.Context(), dao, args[0])
		},
	}
}

func pullCatalogNextCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <oci-reference>",
		Short: "Pull a catalog from an OCI registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			ociService := oci.NewService()
			return catalognext.Pull(cmd.Context(), dao, ociService, args[0])
		},
	}
}

func catalogNextServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers in catalogs",
	}

	cmd.AddCommand(listCatalogNextServersCommand())
	cmd.AddCommand(inspectServerCatalogNextCommand())
	cmd.AddCommand(addCatalogNextServersCommand())
	cmd.AddCommand(removeCatalogNextServersCommand())

	return cmd
}

func inspectServerCatalogNextCommand() *cobra.Command {
	var opts struct {
		Format string
	}

	cmd := &cobra.Command{
		Use:   "inspect <oci-reference> <server-name>",
		Short: "Inspect a server in a catalog",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			supported := slices.Contains(workingset.SupportedFormats(), opts.Format)
			if !supported {
				return fmt.Errorf("unsupported format: %s", opts.Format)
			}
			dao, err := db.New()
			if err != nil {
				return err
			}

			return catalognext.InspectServer(cmd.Context(), dao, args[0], args[1], workingset.OutputFormat(opts.Format))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))
	return cmd
}

func listCatalogNextServersCommand() *cobra.Command {
	var opts struct {
		Filters []string
		Format  string
	}

	cmd := &cobra.Command{
		Use:     "ls <oci-reference>",
		Aliases: []string{"list"},
		Short:   "List servers in a catalog",
		Long: `List all servers in a catalog.

Use --filter to search for servers matching a query (case-insensitive substring matching on server names).
Filters use key=value format (e.g., name=github).`,
		Example: `  # List all servers in a catalog
  docker mcp catalog server ls mcp/docker-mcp-catalog:latest

  # Filter servers by name
  docker mcp catalog server ls mcp/docker-mcp-catalog:latest --filter name=github

  # Combine multiple filters (using short flag)
  docker mcp catalog server ls mcp/docker-mcp-catalog:latest -f name=slack -f name=github

  # Output in JSON format
  docker mcp catalog server ls mcp/docker-mcp-catalog:latest --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			supported := slices.Contains(workingset.SupportedFormats(), opts.Format)
			if !supported {
				return fmt.Errorf("unsupported format: %s", opts.Format)
			}

			dao, err := db.New()
			if err != nil {
				return err
			}

			return catalognext.ListServers(cmd.Context(), dao, args[0], opts.Filters, workingset.OutputFormat(opts.Format))
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVarP(&opts.Filters, "filter", "f", []string{}, "Filter output (e.g., name=github)")
	flags.StringVar(&opts.Format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

	return cmd
}

func addCatalogNextServersCommand() *cobra.Command {
	var servers []string

	cmd := &cobra.Command{
		Use:   "add <oci-reference> [--server <ref1> --server <ref2> ...]",
		Short: "Add MCP servers to a catalog",
		Long:  "Add MCP servers to a catalog using various URI schemes.",
		Example: `  # Add servers from another catalog
  docker mcp catalog server add mcp/my-catalog:latest --server catalog://mcp/docker-mcp-catalog:latest/github

  # Add servers with OCI references
  docker mcp catalog server add mcp/my-catalog:latest --server docker://my-server:latest

  # Add servers with MCP Registry references
  docker mcp catalog server add mcp/my-catalog:latest --server https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

  # Mix server references
  docker mcp catalog server add mcp/my-catalog:latest --server catalog://mcp/docker-mcp-catalog:latest/github --server docker://my-server:latest`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			registryClient := registryapi.NewClient()
			ociService := oci.NewService()
			return catalognext.AddServers(cmd.Context(), dao, registryClient, ociService, args[0], servers)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&servers, "server", []string{}, "Server to include specified with a URI: https:// (MCP Registry reference) or docker:// (Docker Image reference) or catalog:// (Catalog reference) or file:// (Local file path). Can be specified multiple times.")

	return cmd
}

func removeCatalogNextServersCommand() *cobra.Command {
	var names []string

	cmd := &cobra.Command{
		Use:     "remove <oci-reference> --name <name1> --name <name2> ...",
		Aliases: []string{"rm"},
		Short:   "Remove MCP servers from a catalog",
		Long:    "Remove MCP servers from a catalog by server name.",
		Example: `  # Remove servers by name
  docker mcp catalog server remove mcp/my-catalog:latest --name github --name slack

  # Remove a single server
  docker mcp catalog server remove mcp/my-catalog:latest --name github`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return catalognext.RemoveServers(cmd.Context(), dao, args[0], names)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&names, "name", []string{}, "Server name to remove (can be specified multiple times)")

	return cmd
}
