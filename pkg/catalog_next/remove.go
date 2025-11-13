package catalognext

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/google/go-containerregistry/pkg/name"
)

func Remove(ctx context.Context, dao db.DAO, refStr string) error {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse oci-reference %s: %w", refStr, err)
	}

	refStr = oci.FullNameWithoutDigest(ref)

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
	return nil
}
