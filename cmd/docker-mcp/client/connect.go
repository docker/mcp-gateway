package client

import (
	"context"
	"fmt"
)

func Connect(ctx context.Context, cwd string, config Config, vendor string, global, quiet bool) error {
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
	fmt.Println("\033[36mTip: Your client is now connected! Try \033[1;3m'docker mcp tools ls'\033[0;36m to see available tools\033[0m")
	return nil
}
