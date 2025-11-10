package catalognext

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
)

func Push(ctx context.Context, dao db.DAO, digest string, refStr string) error {
	dbCatalog, err := dao.GetCatalog(ctx, digest)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("catalog %s not found", digest)
		}
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}

	if !oci.IsValidInputReference(ref) {
		return fmt.Errorf("reference must be a valid OCI reference")
	}

	catalog := NewFromDb(dbCatalog)

	// Remove source from catalog, since the new source will be the oci artifact
	catalog.Source = ""

	hash, err := oci.PushArtifact(ctx, ref, MCPCatalogArtifactType, catalog, nil)
	if err != nil {
		return fmt.Errorf("failed to push working set artifact: %w", err)
	}

	fmt.Printf("Pushed catalog to %s@sha256:%s\n", oci.FullName(ref), hash)

	return nil
}
