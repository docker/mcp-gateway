package workingset

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

func Pull(ctx context.Context, dao db.DAO, ociService oci.Service, ref string) error {
	telemetry.Init()
	start := time.Now()
	var success bool
	var id string
	defer func() {
		duration := time.Since(start)
		telemetry.RecordWorkingSetOperation(ctx, "pull", id, float64(duration.Milliseconds()), success)
	}()

	workingSet, err := oci.ReadArtifact[WorkingSet](ref, MCPWorkingSetArtifactType)
	if err != nil {
		return fmt.Errorf("failed to read OCI profile: %w", err)
	}

	id, err = createWorkingSetID(ctx, workingSet.Name, dao)
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

	success = true
	return nil
}
