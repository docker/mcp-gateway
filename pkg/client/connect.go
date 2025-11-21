package client

import (
	"context"
	"errors"
	"fmt"
)

var ErrCodexOnlySupportsGlobalConfiguration = errors.New("codex only supports global configuration. Re-run with --global or -g")

func Connect(ctx context.Context, cwd string, config Config, vendor string, global bool, workingSet string) error {
	switch vendor {
	case VendorCodex:
		if !global {
			return ErrCodexOnlySupportsGlobalConfiguration
		}
		if err := ConnectCodex(ctx, workingSet); err != nil {
			return err
		}
	case VendorGordon:
		return fmt.Errorf("gordon support for profiles is not yet implemented")
	default:
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
