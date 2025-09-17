package oauth

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/desktop"
)

func Authorize(ctx context.Context, app string, scopes string) error {
	client := desktop.NewAuthClient()

	authResponse, err := client.PostOAuthApp(ctx, app, scopes, false)
	if err != nil {
		return err
	}

	// Check if the response contains a valid browser URL
	if authResponse.BrowserURL == "" {
		return fmt.Errorf("OAuth provider does not exist")
	}

	fmt.Printf("Opening your browser for authentication. If it doesn't open automatically, please visit: %s\n", authResponse.BrowserURL)

	return nil
}
