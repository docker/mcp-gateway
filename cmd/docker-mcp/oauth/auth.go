package oauth

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/desktop"
)

func NewAuthorizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "authorize <app>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthorize(cmd.Context(), args[0])
		},
	}
}

func runAuthorize(ctx context.Context, app string) error {
	client := desktop.NewAuthClient()

	authResponse, err := client.PostOAuthApp(ctx, app, "")
	if err != nil {
		return err
	}

	fmt.Printf("Opening your browser for authentication. If it doesn't open automatically, please visit: %s\n", authResponse.BrowserURL)

	return nil
}
