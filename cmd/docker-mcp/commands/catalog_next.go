package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	catalognext "github.com/docker/mcp-gateway/pkg/catalog_next"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
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

	return cmd
}

func createCatalogNextCommand() *cobra.Command {
	var opts struct {
		Name              string
		FromWorkingSet    string
		FromLegacyCatalog string
		RemoveExisting    bool
	}

	cmd := &cobra.Command{
		Use:   "create [--from-working-set <working-set-id>] [--from-legacy-catalog <url>] [--name <name>] [--remove-existing]",
		Short: "Create a new catalog from a working set or legacy catalog",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.FromWorkingSet == "" && opts.FromLegacyCatalog == "" {
				return fmt.Errorf("either --from-working-set or --from-legacy-catalog must be provided")
			}
			if opts.FromWorkingSet != "" && opts.FromLegacyCatalog != "" {
				return fmt.Errorf("cannot use both --from-working-set and --from-legacy-catalog")
			}

			dao, err := db.New()
			if err != nil {
				return err
			}
			return catalognext.Create(cmd.Context(), dao, opts.FromWorkingSet, opts.FromLegacyCatalog, opts.Name, opts.RemoveExisting)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.FromWorkingSet, "from-working-set", "", "Working set ID to create the catalog from")
	flags.StringVar(&opts.FromLegacyCatalog, "from-legacy-catalog", "", "Legacy catalog URL to create the catalog from")
	flags.StringVar(&opts.Name, "name", "", "Name of the catalog")
	flags.BoolVar(&opts.RemoveExisting, "remove-existing", false, "Remove existing catalogs that come from the same source or have the same digest")

	return cmd
}

func showCatalogNextCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)

	cmd := &cobra.Command{
		Use:   "show <catalog-digest>",
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
			return catalognext.Show(cmd.Context(), dao, args[0], workingset.OutputFormat(format))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

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
		Use:     "remove <catalog-digest>",
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
		Use:   "push <catalog-digest> <oci-reference>",
		Short: "Push a catalog to an OCI registry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return catalognext.Push(cmd.Context(), dao, args[0], args[1])
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
