package workingset

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
)

func Pull(ctx context.Context, dao db.DAO, ociService oci.Service, ref string) error {
	workingSet, err := oci.ReadArtifact[WorkingSet](ref, MCPWorkingSetArtifactType)
	if err != nil {
		return fmt.Errorf("failed to read OCI profile: %w", err)
	}

	id, err := createWorkingSetID(ctx, workingSet.Name, dao)
	if err != nil {
		return fmt.Errorf("failed to create profile id: %w", err)
	}
	workingSet.ID = id

	// Resolve snapshots for each server before saving
	for i := range len(workingSet.Servers) {
		if workingSet.Servers[i].Snapshot == nil {
			snapshot, err := ResolveSnapshot(ctx, ociService, workingSet.Servers[i])
			if err != nil {
				return fmt.Errorf("failed to resolve snapshot for server[%d]: %w", i, err)
			}
			workingSet.Servers[i].Snapshot = snapshot
		}
	}

	RegisterOAuthProvidersForServers(ctx, workingSet.Servers)

	if err := workingSet.Validate(); err != nil {
		return fmt.Errorf("invalid profile: %w", err)
	}

	err = dao.CreateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}

	fmt.Printf("Profile %s imported as %s\n", workingSet.Name, id)

	return nil
}
