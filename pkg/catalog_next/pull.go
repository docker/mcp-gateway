package catalognext

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/telemetry"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func Pull(ctx context.Context, dao db.DAO, ociService oci.Service, refStr string) error {
	telemetry.Init()
	start := time.Now()
	var success bool
	defer func() {
		duration := time.Since(start)
		telemetry.RecordCatalogOperation(ctx, "pull", refStr, float64(duration.Milliseconds()), success)
	}()
	catalog, err := pullCatalog(ctx, dao, ociService, refStr)
	if err != nil {
		return err
	}

	fmt.Printf("Catalog %s pulled\n", catalog.Ref)

	success = true
	return nil
}

func pullCatalog(ctx context.Context, dao db.DAO, ociService oci.Service, refStr string) (*db.Catalog, error) {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OCI reference %s: %w", refStr, err)
	}
	source := oci.FullName(ref)

	catalogArtifact, err := oci.ReadArtifact[CatalogArtifact](refStr, MCPCatalogArtifactType)
	if err != nil {
		return nil, fmt.Errorf("failed to read OCI catalog: %w", err)
	}

	catalog := Catalog{
		CatalogArtifact: catalogArtifact,
		Ref:             oci.FullNameWithoutDigest(ref),
		Source:          SourcePrefixOCI + source,
	}

	// Resolve any unresolved snapshots first
	for i := range len(catalog.Servers) {
		if catalog.Servers[i].Snapshot != nil {
			continue
		}
		switch catalog.Servers[i].Type {
		case workingset.ServerTypeImage:
			serverSnapshot, err := workingset.ResolveImageSnapshot(ctx, ociService, catalog.Servers[i].Image)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve image snapshot: %w", err)
			}
			catalog.Servers[i].Snapshot = serverSnapshot
		case workingset.ServerTypeRegistry:
			// TODO(cody): Ignore until supported
		}
	}

	if err := catalog.Validate(); err != nil {
		return nil, fmt.Errorf("invalid catalog: %w", err)
	}

	dbCatalog, err := catalog.ToDb()
	if err != nil {
		return nil, fmt.Errorf("failed to convert catalog to db: %w", err)
	}

	err = dao.UpsertCatalog(ctx, dbCatalog)
	if err != nil {
		return nil, fmt.Errorf("failed to create catalog: %w", err)
	}

	return &dbCatalog, nil
}
