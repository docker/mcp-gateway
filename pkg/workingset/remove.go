package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
)

func Remove(ctx context.Context, dao db.DAO, id string) error {
	_, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("working set %s not found", id)
		}
		return fmt.Errorf("failed to get working set: %w", err)
	}

	err = dao.RemoveWorkingSet(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to remove working set: %w", err)
	}

	fmt.Printf("Removed working set %s\n", id)
	return nil
}
