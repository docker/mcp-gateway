package oauth

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

func Revoke(ctx context.Context, app string) error {
	client := desktop.NewAuthClient()

	return client.DeleteOAuthApp(ctx, app)
}
