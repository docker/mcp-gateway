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

func Tag(ctx context.Context, dao db.DAO, refStr string, tag string) error {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse oci-reference %s: %w", refStr, err)
	}

	refStr = oci.FullNameWithoutDigest(ref)

	tagRef, err := name.ParseReference(tag)
	if err != nil {
		return fmt.Errorf("failed to parse tag %s: %w", tag, err)
	}
	tag = oci.FullNameWithoutDigest(tagRef)

	dbCatalog, err := dao.GetCatalog(ctx, refStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("catalog %s not found", refStr)
		}
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	dbCatalog.Source = SourcePrefixOCI + dbCatalog.Ref
	dbCatalog.Ref = tag

	err = dao.UpsertCatalog(ctx, *dbCatalog)
	if err != nil {
		return fmt.Errorf("failed to tag catalog: %w", err)
	}

	fmt.Printf("Tagged catalog %s as %s\n", refStr, tag)
	return nil
}
