package catalognext

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/telemetry"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

// Pull pulls a catalog from its source (OCI registry or community API)
// It auto-detects well-known community registries like "registry.modelcontextprotocol.io"
func Pull(ctx context.Context, dao db.DAO, ociService oci.Service, refStr string) error {
	telemetry.Init()
	start := time.Now()
	var success bool
	defer func() {
		duration := time.Since(start)
		telemetry.RecordCatalogOperation(ctx, "pull", refStr, float64(duration.Milliseconds()), success)
	}()

	// Check if this is a well-known community registry
	if IsAPIRegistry(refStr) {
		result, err := PullCommunity(ctx, dao, refStr, DefaultPullCommunityOptions())
		if err != nil {
			return err
		}
		printRegistryPullResult(refStr, result)
		success = true
		return nil
	}

	// OCI pull
	catalog, err := pullOCI(ctx, dao, ociService, refStr)
	if err != nil {
		return err
	}

	fmt.Printf("Catalog %s pulled\n", catalog.Ref)

	success = true
	return nil
}

// pullOCI pulls a catalog from an OCI registry
func pullOCI(ctx context.Context, dao db.DAO, ociService oci.Service, refStr string) (*db.Catalog, error) {
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

	err = dao.RecordPull(ctx, refStr)
	if err != nil {
		return nil, fmt.Errorf("failed to record pull record: %w", err)
	}

	return &dbCatalog, nil
}

// pullCatalog is kept for compatibility with show.go
func pullCatalog(ctx context.Context, dao db.DAO, ociService oci.Service, refStr string) error {
	// Check if this is a well-known community registry
	if IsAPIRegistry(refStr) {
		_, err := PullCommunity(ctx, dao, refStr, DefaultPullCommunityOptions())
		return err
	}

	_, err := pullOCI(ctx, dao, ociService, refStr)
	return err
}

func printRegistryPullResult(refStr string, result *PullCommunityResult) {
	fmt.Printf("Pulled %d servers from %s\n", result.ServersAdded, refStr)
	fmt.Printf("  Total in registry: %d\n", result.TotalServers)
	fmt.Printf("  Imported:          %d\n", result.ServersAdded)
	fmt.Printf("    OCI (stdio):     %d\n", result.ServersOCI)
	fmt.Printf("    Remote:          %d\n", result.ServersRemote)
	fmt.Printf("  Skipped:           %d\n", result.ServersSkipped)

	// Print skipped breakdown sorted by count (descending)
	if len(result.SkippedByType) > 0 {
		type typeCount struct {
			name  string
			count int
		}
		var sorted []typeCount
		for t, c := range result.SkippedByType {
			sorted = append(sorted, typeCount{t, c})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].count > sorted[j].count
		})
		for _, tc := range sorted {
			label := tc.name
			if label == "none" {
				label = "no packages"
			}
			fmt.Printf("    %-17s%d\n", label+":", tc.count)
		}
	}
}
