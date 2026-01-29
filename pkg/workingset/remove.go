package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/docker/mcp-gateway/pkg/client"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

func Remove(ctx context.Context, dao db.DAO, cwd string, id string) error {
	telemetry.Init()
	start := time.Now()
	var success bool
	defer func() {
		duration := time.Since(start)
		telemetry.RecordWorkingSetOperation(ctx, "remove", id, float64(duration.Milliseconds()), success)
	}()
	_, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("profile %s not found", id)
		}
		return fmt.Errorf("failed to get profile: %w", err)
	}

	err = dao.RemoveWorkingSet(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to remove profile: %w", err)
	}

	if err := removeClientsByProfile(ctx, cwd, id); err != nil {
		fmt.Fprintf(os.Stderr, "warning: unable to remove client connections for the profile: %v\n", err)
	}

	fmt.Printf("Removed profile %s\n", id)
	success = true
	return nil
}

func removeClientsByProfile(ctx context.Context, cwd string, id string) error {
	cfg := client.ReadConfig()

	clients := client.FindClientsByProfile(ctx, id)
	for vendor := range clients {
		if err := client.Disconnect(ctx, cwd, *cfg, vendor, true); err != nil {
			return err
		}
	}
	return nil
}
