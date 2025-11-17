package client

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/client"
)

func Disconnect(ctx context.Context, cwd string, config client.Config, vendor string, global, quiet bool) error {
	if vendor == client.VendorCodex {
		if !global {
			return fmt.Errorf("codex only supports global configuration. Re-run with --global or -g")
		}
		if err := client.DisconnectCodex(ctx); err != nil {
			return err
		}
	} else if vendor == client.VendorGordon && global {
		if err := client.DisconnectGordon(ctx); err != nil {
			return err
		}
	} else {
		updater, err := client.GetUpdater(vendor, global, cwd, config)
		if err != nil {
			return err
		}
		if err := updater(client.DockerMCPCatalog, nil); err != nil {
			return err
		}
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
