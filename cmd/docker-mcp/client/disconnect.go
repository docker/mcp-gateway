package client

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/client"
)

func Disconnect(ctx context.Context, cwd string, config client.Config, vendor string, global, quiet bool) error {
	if err := client.Disconnect(ctx, cwd, config, vendor, global); err != nil {
		return err
	}
	if quiet {
		return nil
	}
	if err := List(ctx, cwd, config, global, false); err != nil {
		return err
	}
	fmt.Printf("You might have to restart '%s'.\n", vendor)
	return nil
}
