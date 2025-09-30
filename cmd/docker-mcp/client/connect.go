package client

import (
	"context"
	"fmt"
)

func Connect(ctx context.Context, cwd string, config Config, vendor string, global, quiet bool) error {
	if vendor == vendorGordon && global {
		if err := connectGordon(ctx); err != nil {
			return err
		}
	} else if vendor == vendorCodex && global {
		if err := connectCodex(ctx); err != nil {
			return err
		}
	} else {
		updater, err := GetUpdater(vendor, global, cwd, config)
		if err != nil {
			return err
		}
		if err := updater(DockerMCPCatalog, newMCPGatewayServer()); err != nil {
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
