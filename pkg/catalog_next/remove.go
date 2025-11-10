package catalognext

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
)

func Remove(ctx context.Context, dao db.DAO, digest string) error {
	_, err := dao.GetCatalog(ctx, digest)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("catalog %s not found", digest)
		}
		return fmt.Errorf("failed to remove catalog: %w", err)
	}

	err = dao.DeleteCatalog(ctx, digest)
	if err != nil {
		return fmt.Errorf("failed to remove catalog: %w", err)
	}

	fmt.Printf("Removed catalog %s\n", digest)
	return nil
}
