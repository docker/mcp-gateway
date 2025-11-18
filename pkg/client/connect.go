package client

import (
	"context"
	"errors"
	"fmt"
)

var ErrCodexOnlySupportsGlobalConfiguration = errors.New("codex only supports global configuration. Re-run with --global or -g")

func Connect(ctx context.Context, cwd string, config Config, vendor string, global bool, workingSet string) error {
	if vendor == VendorCodex {
		if !global {
			return ErrCodexOnlySupportsGlobalConfiguration
		}
		if err := ConnectCodex(ctx, workingSet); err != nil {
			return err
		}
	} else if vendor == VendorGordon && global {
		if workingSet != "" {
			// Gordon doesn't support profiles yet
			return fmt.Errorf("gordon cannot be connected to a profile")
		}
		if err := ConnectGordon(ctx); err != nil {
			return err
		}
	} else {
		updater, err := getUpdater(vendor, global, cwd, config)
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
	return nil
}
