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
		Use:   "catalog-next",
		Short: "Manage catalogs (next generation)",
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
		Title             string
		FromWorkingSet    string
		FromLegacyCatalog string
		Servers           []string
	}

	cmd := &cobra.Command{
		Use:   "create <oci-reference> [--server <ref1> --server <ref2> ...] [--from-profile <profile-id>] [--from-legacy-catalog <url>] [--title <title>]",
		Short: "Create a new catalog from a profile or legacy catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.FromWorkingSet != "" && opts.FromLegacyCatalog != "" {
				return fmt.Errorf("cannot use both --from-profile and --from-legacy-catalog")
			}

			dao, err := db.New()
			if err != nil {
				return err
			}
			registryClient := registryapi.NewClient()
			ociService := oci.NewService()
			return catalognext.Create(cmd.Context(), dao, registryClient, ociService, args[0], opts.Servers, opts.FromWorkingSet, opts.FromLegacyCatalog, opts.Title)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVar(&opts.Servers, "server", []string{}, "Server to include specified with a URI: https:// (MCP Registry reference) or docker:// (Docker Image reference) or catalog:// (Catalog reference). Can be specified multiple times.")
	flags.StringVar(&opts.FromWorkingSet, "from-profile", "", "Profile ID to create the catalog from")
	flags.StringVar(&opts.FromLegacyCatalog, "from-legacy-catalog", "", "Legacy catalog URL to create the catalog from")
	flags.StringVar(&opts.Title, "title", "", "Title of the catalog")

	return cmd
}

func tagCatalogNextCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tag <oci-reference> <tag>",
		Short: "Tag a catalog",
		Args:  cobra.ExactArgs(2),
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
  docker mcp catalog-next server ls mcp/docker-mcp-catalog:latest

  # Filter servers by name
  docker mcp catalog-next server ls mcp/docker-mcp-catalog:latest --filter name=github

  # Combine multiple filters (using short flag)
  docker mcp catalog-next server ls mcp/docker-mcp-catalog:latest -f name=slack -f name=github

  # Output in JSON format
  docker mcp catalog-next server ls mcp/docker-mcp-catalog:latest --format json`,
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
