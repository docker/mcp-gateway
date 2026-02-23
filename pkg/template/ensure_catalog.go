package template

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	catalognext "github.com/docker/mcp-gateway/pkg/catalog_next"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
)

// EnsureCatalogExists checks whether the Docker MCP catalog is available
// locally and pulls it if it is not. This allows template commands to work
// out of the box without requiring the user to manually pull the catalog
// first.
func EnsureCatalogExists(ctx context.Context, dao db.DAO, ociService oci.Service) error {
	_, err := dao.GetCatalog(ctx, DefaultCatalogRef)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check for catalog: %w", err)
	}

	fmt.Println("Pulling Docker MCP catalog...")
	if err := catalognext.Pull(ctx, dao, ociService, DefaultCatalogRef); err != nil {
		return fmt.Errorf("failed to pull catalog %s: %w", DefaultCatalogRef, err)
	}

	return nil
}
