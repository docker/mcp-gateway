package oauth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/formatting"
)

func Ls(ctx context.Context, outputJSON bool) error {
	client := desktop.NewAuthClient()

	apps, err := client.ListOAuthApps(ctx)
	if err != nil {
		return err
	}

	if outputJSON {
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
