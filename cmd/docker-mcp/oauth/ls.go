package oauth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/formatting"
)

func Ls(ctx context.Context, outputJSON bool) error {
	client := desktop.NewAuthClient()

	// Get OAuth apps from pinata
	apps, err := client.ListOAuthApps(ctx)
	if err != nil {
		return err
	}

	// Add remote MCP servers with OAuth to the list
	catalog, err := catalog.GetWithOptions(ctx, true, nil)
	if err == nil {
		for serverName, server := range catalog.Servers {
			if server.OAuth != nil && server.OAuth.Enabled {
				// Check if the provider is authorized
				providerAuthorized := false
				for _, app := range apps {
					if app.App == server.OAuth.Provider && app.Authorized {
						providerAuthorized = true
						break
					}
				}
				apps = append(apps, desktop.OAuthApp{
					App:        serverName,
					Authorized: providerAuthorized,
					Provider:   server.OAuth.Provider,
				})
			}
		}
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
