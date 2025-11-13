package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
)

func Push(ctx context.Context, dao db.DAO, id string, refStr string) error {
	dbSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("working set %s not found", id)
		}
		return fmt.Errorf("failed to get working set: %w", err)
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
		return fmt.Errorf("failed to push working set artifact: %w", err)
	}

	fmt.Printf("Pushed working set to %s@sha256:%s\n", oci.FullName(ref), hash)

	return nil
}
