package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	catalognext "github.com/docker/mcp-gateway/pkg/catalog_next"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/migrate"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func catalogNextCommand(docker docker.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog-next",
		Short: "Manage catalogs (next generation)",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			defer dao.Close()
			migrate.MigrateConfig(cmd.Context(), docker, dao)
			return nil
		},
	}

	cmd.AddCommand(createCatalogNextCommand())
	cmd.AddCommand(showCatalogNextCommand())
	cmd.AddCommand(listCatalogNextCommand())
	cmd.AddCommand(removeCatalogNextCommand())
	cmd.AddCommand(pushCatalogNextCommand())
	cmd.AddCommand(pullCatalogNextCommand())
	cmd.AddCommand(tagCatalogNextCommand())

	return cmd
}

func createCatalogNextCommand() *cobra.Command {
	var opts struct {
		Title             string
		FromWorkingSet    string
		FromLegacyCatalog string
	}

	cmd := &cobra.Command{
		Use:   "create <oci-reference> [--from-profile <profile-id>] [--from-legacy-catalog <url>] [--title <title>]",
		Short: "Create a new catalog from a profile or legacy catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.FromWorkingSet == "" && opts.FromLegacyCatalog == "" {
				return fmt.Errorf("either --from-profile or --from-legacy-catalog must be provided")
			}
			if opts.FromWorkingSet != "" && opts.FromLegacyCatalog != "" {
				return fmt.Errorf("cannot use both --from-profile and --from-legacy-catalog")
			}

			dao, err := db.New()
			if err != nil {
				return err
			}
			return catalognext.Create(cmd.Context(), dao, args[0], opts.FromWorkingSet, opts.FromLegacyCatalog, opts.Title)
		},
	}

	flags := cmd.Flags()
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
			ociService := oci.NewService()
			return catalognext.Show(cmd.Context(), dao, ociService, args[0], workingset.OutputFormat(format), pullOption)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))
	flags.StringVar(&pullOption, "pull", string(catalognext.PullOptionNever), fmt.Sprintf("Supported: %s, or duration (e.g. '1h', '1d'). Duration represents time since last update.", strings.Join(catalognext.SupportedPullOptions(), ", ")))
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
