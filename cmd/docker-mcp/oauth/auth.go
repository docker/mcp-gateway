package oauth

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

func Authorize(ctx context.Context, app string, scopes string) error {
	client := desktop.NewAuthClient()

	authResponse, err := client.PostOAuthApp(ctx, app, scopes, false)
	if err != nil {
		return err
	}

	fmt.Printf("Opening your browser for authentication. If it doesn't open automatically, please visit: %s\n", authResponse.BrowserURL)

	return nil
}
