package oauth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/formatting"
	"github.com/docker/mcp-gateway/pkg/desktop"
	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
)

func Ls(ctx context.Context, outputJSON bool) error {
	if pkgoauth.IsCEMode() {
		return lsCEMode(ctx, outputJSON)
	}
	return lsDesktopMode(ctx, outputJSON)
}

// lsDesktopMode lists OAuth apps via Docker Desktop (existing behavior)
func lsDesktopMode(ctx context.Context, outputJSON bool) error {
	client := desktop.NewAuthClient()

	// Get OAuth apps from Docker Desktop (includes both built-in and DCR providers)
	// DCR providers are created by 'docker mcp server enable' (unregistered) and registered by 'docker mcp oauth authorize'
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

// lsCEMode lists OAuth apps in standalone CE mode using local credential storage
func lsCEMode(_ context.Context, outputJSON bool) error {
	credHelper := pkgoauth.NewReadWriteCredentialHelper()
	manager := pkgoauth.NewManager(credHelper)

	clients, err := manager.ListDCRClients()
	if err != nil {
		return fmt.Errorf("failed to list OAuth apps: %w", err)
	}

	type ceApp struct {
		App        string `json:"app"`
		Authorized bool   `json:"authorized"`
		Provider   string `json:"provider,omitempty"`
	}

	var apps []ceApp
	for name, client := range clients {
		apps = append(apps, ceApp{
			App:        name,
			Authorized: manager.HasValidToken(name),
			Provider:   client.ProviderName,
		})
	}

	if outputJSON {
		if len(apps) == 0 {
			apps = make([]ceApp, 0)
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
