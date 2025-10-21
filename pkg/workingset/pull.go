package workingset

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
)

func Pull(ctx context.Context, dao db.DAO, ref string) error {
	ociCatalog, err := oci.ReadArtifact[Catalog](ref, MCPCatalogArtifactType)
	if err != nil {
		return fmt.Errorf("failed to read OCI working set: %w", err)
	}

	// TODO: validate the catalog
	workingSet := ociCatalog.ToWorkingSet()

	id, err := createWorkingSetId(ctx, workingSet.Name, dao)
	if err != nil {
		return fmt.Errorf("failed to create working set id: %w", err)
	}
	workingSet.ID = id

	err = dao.CreateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to create working set: %w", err)
	}

	fmt.Printf("Working set %s imported as %s\n", workingSet.Name, id)

	return nil
}
