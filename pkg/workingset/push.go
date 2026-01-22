package workingset

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

func Push(ctx context.Context, dao db.DAO, id string, refStr string) error {
	telemetry.Init()
	start := time.Now()
	var success bool
	defer func() {
		duration := time.Since(start)
		telemetry.RecordWorkingSetOperation(ctx, "push", id, float64(duration.Milliseconds()), success)
	}()
	dbSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("profile %s not found", id)
		}
		return fmt.Errorf("failed to get profile: %w", err)
	}

	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}

	if !oci.IsValidInputReference(ref) {
		return fmt.Errorf("reference must be a valid OCI reference")
	}

	workingSet := NewFromDb(dbSet)

	hash, err := oci.PushArtifact(ctx, ref, MCPWorkingSetArtifactType, workingSet, nil)
	if err != nil {
		return fmt.Errorf("failed to push profile artifact: %w", err)
	}

	fmt.Printf("Pushed profile to %s@sha256:%s\n", oci.FullName(ref), hash)

	success = true
	return nil
}
