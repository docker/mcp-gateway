package catalognext

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/google/go-containerregistry/pkg/name"
)

func Pull(ctx context.Context, dao db.DAO, ociService oci.Service, refStr string) error {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse OCI reference %s: %w", ref, err)
	}
	source := oci.FullName(ref)

	catalog, err := oci.ReadArtifact[Catalog](refStr, MCPCatalogArtifactType)
	if err != nil {
		return fmt.Errorf("failed to read OCI catalog: %w", err)
	}

	catalog.Source = "oci:" + source

	// Resolve any unresolved snapshots first
	for i := range len(catalog.Servers) {
		if catalog.Servers[i].Snapshot != nil {
			continue
		}
		switch catalog.Servers[i].Type {
		case workingset.ServerTypeImage:
			serverSnapshot, err := workingset.ResolveImageSnapshot(ctx, ociService, catalog.Servers[i].Image)
			if err != nil {
				return fmt.Errorf("failed to resolve image snapshot: %w", err)
			}
			catalog.Servers[i].Snapshot = serverSnapshot
		case workingset.ServerTypeRegistry:
			// TODO(cody): Ignore until supported
		}
	}

	if err := catalog.Validate(); err != nil {
		return fmt.Errorf("invalid catalog: %w", err)
	}

	err = dao.CreateCatalog(ctx, catalog.ToDb())
	if err != nil {
		if db.IsDuplicateDigestError(err) {
			return fmt.Errorf("catalog with digest %s already exists", catalog.Digest())
		}
		return fmt.Errorf("failed to create catalog: %w", err)
	}

	fmt.Printf("Catalog %s imported with digest %s\n", catalog.Name, catalog.Digest())

	return nil
}
