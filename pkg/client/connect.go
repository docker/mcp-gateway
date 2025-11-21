package client

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
)

var (
	ErrCodexOnlySupportsGlobalConfiguration = errors.New("codex only supports global configuration. Re-run with --global or -g")
	newDAO                                  = db.New
)

func Connect(ctx context.Context, cwd string, config Config, vendor string, global bool, workingSet string) error {
	if workingSet != "" {
		dao, err := newDAO()
		if err != nil {
			return fmt.Errorf("failed to create database client: %w", err)
		}
		defer dao.Close()

		_, err = dao.GetWorkingSet(ctx, workingSet)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("profile '%s' not found", workingSet)
			}
			return fmt.Errorf("failed to get profile: %w", err)
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
