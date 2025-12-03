package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
)

var ErrCodexOnlySupportsGlobalConfiguration = errors.New("codex only supports global configuration. Re-run with --global or -g")

func Connect(ctx context.Context, dao db.DAO, cwd string, config Config, vendor string, global bool, workingSet string) error {
	if workingSet != "" {
		_, err := dao.GetWorkingSet(ctx, workingSet)
		if err != nil {
			return fmt.Errorf("failed to get profile: %s", workingSet)
		}
	}

	if vendor == VendorCodex {
		if !global {
			return ErrCodexOnlySupportsGlobalConfiguration
		}
		if err := ConnectCodex(ctx, workingSet); err != nil {
			return err
		}
	} else if vendor == VendorGordon && global {
		if err := ConnectGordon(ctx, workingSet); err != nil {
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
