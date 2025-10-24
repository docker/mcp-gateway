package client

import (
	"context"
	"fmt"
)

func Connect(ctx context.Context, cwd string, config Config, vendor string, global, quiet bool, workingSet string) error {
	if vendor == vendorCodex {
		if !global {
			return fmt.Errorf("codex only supports global configuration. Re-run with --global or -g")
		}
		if err := connectCodex(ctx); err != nil {
			return err
		}
	} else if vendor == vendorGordon && global {
		if err := connectGordon(ctx); err != nil {
			return err
		}
	} else {
		updater, err := GetUpdater(vendor, global, cwd, config)
		if err != nil {
			return err
		}
		if workingSet != "" {
			if err := updater(DockerMCPCatalog, newMcpGatewayServerWithWorkingSet(workingSet)); err != nil {
				return err
			}
		} else {
			if err := updater(DockerMCPCatalog, newMCPGatewayServer()); err != nil {
				return err
			}
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
