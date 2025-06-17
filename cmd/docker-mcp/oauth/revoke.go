package oauth

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/desktop"
)

func NewRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "revoke <app>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRevoke(cmd.Context(), args[0])
		},
	}
}

func runRevoke(ctx context.Context, app string) error {
	client := desktop.NewAuthClient()

	return client.DeleteOAuthApp(ctx, app)
}
