package catalognext

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

func Remove(ctx context.Context, dao db.DAO, refStr string) error {
	telemetry.Init()
	start := time.Now()
	var success bool
	defer func() {
		duration := time.Since(start)
		telemetry.RecordCatalogOperation(ctx, "remove", refStr, float64(duration.Milliseconds()), success)
	}()
	resolved, err := resolveCatalogRef(refStr)
	if err != nil {
		return err
	}
	refStr = resolved

	_, err = dao.GetCatalog(ctx, refStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("catalog %s not found", refStr)
		}
		return fmt.Errorf("failed to remove catalog: %w", err)
	}

	err = dao.DeleteCatalog(ctx, refStr)
	if err != nil {
		return fmt.Errorf("failed to remove catalog: %w", err)
	}

	fmt.Printf("Removed catalog %s\n", refStr)
	success = true
	return nil
}
