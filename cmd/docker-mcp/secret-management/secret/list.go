package secret

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/formatting"
)

type listOptions struct {
	JSON bool
}

func ListCommand() *cobra.Command {
	opts := listOptions{}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all secret names in Docker Desktop's secret store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runList(cmd.Context(), opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.JSON, "json", false, "Print as JSON.")
	return cmd
}

func runList(ctx context.Context, opts listOptions) error {
	l, err := desktop.NewSecretsClient().ListJfsSecrets(ctx)
	if err != nil {
		return err
	}

	if opts.JSON {
		if len(l) == 0 {
			l = []desktop.StoredSecret{} // Guarantee empty list (instead of displaying null)
		}
		jsonData, err := json.MarshalIndent(l, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(jsonData))
		return nil
	}
	var rows [][]string
	for _, v := range l {
		rows = append(rows, []string{v.Name, v.Provider})
	}
	formatting.PrettyPrintTable(rows, []int{40, 120})
	return nil
}
