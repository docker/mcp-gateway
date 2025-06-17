package oauth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/desktop"
	"github.com/docker/docker-mcp/cmd/docker-mcp/secret-management/formatting"

	"github.com/spf13/cobra"
)

type listOptions struct {
	JSON bool
}

func NewLsCmd() *cobra.Command {
	opts := listOptions{}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List available OAuth apps.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLs(cmd.Context(), opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.JSON, "json", false, "Print as JSON.")
	return cmd
}

func runLs(ctx context.Context, opts listOptions) error {
	client := desktop.NewAuthClient()

	apps, err := client.ListOAuthApps(ctx)
	if err != nil {
		return err
	}

	if opts.JSON {
		if len(apps) == 0 {
			apps = make([]desktop.OAuthApp, 0) // Guarantee empty list (instead of displaying null)
		}
		jsonData, err := json.MarshalIndent(apps, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(jsonData))
		return nil
	}
	var rows [][]string
	for _, app := range apps {
		authorized := "not authorized"
		if app.Authorized {
			authorized = "authorized"
		}
		rows = append(rows, []string{app.App, authorized})
	}
	formatting.PrettyPrintTable(rows, []int{80, 120})
	return nil
}
