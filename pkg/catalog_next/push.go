package catalognext

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

func Push(ctx context.Context, dao db.DAO, refStr string) error {
	telemetry.Init()
	start := time.Now()
	var success bool
	defer func() {
		duration := time.Since(start)
		telemetry.RecordCatalogOperation(ctx, "push", refStr, float64(duration.Milliseconds()), success)
	}()
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}

	if !oci.IsValidInputReference(ref) {
		return fmt.Errorf("reference must be a valid OCI reference without a digest")
	}

	refStr = oci.FullNameWithoutDigest(ref)

	dbCatalog, err := dao.GetCatalog(ctx, refStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("catalog %s not found", refStr)
		}
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	catalog := NewFromDb(dbCatalog)

	hash, err := oci.PushArtifact(ctx, ref, MCPCatalogArtifactType, catalog.CatalogArtifact, nil)
	if err != nil {
		return fmt.Errorf("failed to push catalog artifact: %w", err)
	}

	fmt.Printf("Pushed catalog to %s@sha256:%s\n", oci.FullName(ref), hash)

	success = true
	return nil
}
