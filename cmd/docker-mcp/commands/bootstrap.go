package commands

import (
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
)

func bootstrapCatalogCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap <output-file-path>",
		Short: "Create a starter catalog file with Docker and Docker Hub server entries as examples",
		Long: `Create a starter catalog file with Docker Hub and Docker CLI server entries as examples.
This command extracts the official Docker server definitions and creates a properly formatted
catalog file that users can modify and use as a foundation for their custom catalogs.

The output file is standalone and not automatically imported - users can modify it and then
import it or use it as a source for the 'catalog add' command.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return catalog.Bootstrap(cmd.Context(), args[0])
		},
	}
}
