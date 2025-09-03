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

	// Get OAuth apps from Docker Desktop (includes DCR providers automatically registered)
	apps, err := client.ListOAuthApps(ctx)
	if err != nil {
		return err
	}

	// Create a map to track existing apps to prevent duplicates.
	// CONTEXT: After DCR implementation, remote MCP servers (like notion-remote, 
	// huggingface-remote) exist in BOTH places:
	// 1. As Docker Desktop OAuth apps (from automatic DCR registration)
	// 2. As catalog OAuth servers (from remote-mcp-catalog.yaml with oauth.enabled: true)
	// 
	// Before DCR, these were mutually exclusive data sources, but now they overlap.
	// We need to deduplicate to prevent showing servers twice in the list.
	existingApps := make(map[string]bool)
	for _, app := range apps {
		existingApps[app.App] = true
	}

	// Add catalog OAuth servers only if they don't already exist as OAuth apps.
	// This preserves backward compatibility for non-DCR OAuth servers while 
	// preventing duplicates for DCR-enabled servers.
	catalog, err := catalog.GetWithOptions(ctx, true, nil)
	if err == nil {
		for serverName, server := range catalog.Servers {
			if server.OAuth != nil && server.OAuth.Enabled {
				// Skip if this server already exists as an OAuth app (prevents duplicates).
				// DCR providers automatically register in Docker Desktop, so we prioritize 
				// that source since it has actual authorization status and tokens.
				if existingApps[serverName] {
					continue
				}

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
				
				// Track this app to prevent future duplicates
				existingApps[serverName] = true
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
