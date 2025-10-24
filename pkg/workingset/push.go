package workingset

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/google/go-containerregistry/pkg/name"
)

func Push(ctx context.Context, dao db.DAO, id string, refStr string) error {
	dbSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get working set: %w", err)
	}
	if dbSet == nil {
		return fmt.Errorf("working set %s not found", id)
	}

	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}

	if !isValidInputReference(ref) {
		return fmt.Errorf("reference must be a valid OCI reference")
	}

	workingSet := NewFromDb(dbSet)
	catalog := NewCatalogFromWorkingSet(workingSet)

	hash, err := oci.PushArtifact(ctx, ref, MCPCatalogArtifactType, catalog, nil)
	if err != nil {
		return fmt.Errorf("failed to push working set artifact: %w", err)
	}

	fmt.Printf("Pushed working set to %s@sha256:%s\n", fullName(ref), hash)

	return nil
}
