package catalognext

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	legacycatalog "github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func Create(ctx context.Context, dao db.DAO, workingSetID string, legacyCatalogURL string, name string, removeExisting bool) error {
	var catalog Catalog
	var err error
	if workingSetID != "" {
		catalog, err = createCatalogFromWorkingSet(ctx, dao, workingSetID)
		if err != nil {
			return fmt.Errorf("failed to create catalog from working set: %w", err)
		}
	} else if legacyCatalogURL != "" {
		catalog, err = createCatalogFromLegacyCatalog(ctx, legacyCatalogURL)
		if err != nil {
			return fmt.Errorf("failed to create catalog from legacy catalog: %w", err)
		}
	} else {
		return fmt.Errorf("either working set ID or legacy catalog URL must be provided")
	}

	if name != "" {
		catalog.Name = name
	}

	if err := catalog.Validate(); err != nil {
		return fmt.Errorf("invalid catalog: %w", err)
	}

	if removeExisting {
		err = dao.DeleteCatalogBySource(ctx, catalog.Source)
		if err != nil {
			return fmt.Errorf("failed to delete existing catalog: %w", err)
		}
		err = dao.DeleteCatalog(ctx, catalog.Digest())
		if err != nil {
			return fmt.Errorf("failed to delete existing catalog: %w", err)
		}
	}

	err = dao.CreateCatalog(ctx, catalog.ToDb())
	if err != nil {
		if db.IsDuplicateDigestError(err) {
			return fmt.Errorf("catalog with digest %s already exists", catalog.Digest())
		}
		return fmt.Errorf("failed to create catalog: %w", err)
	}

	fmt.Printf("Catalog %s created with digest %s\n", catalog.Name, catalog.Digest())

	return nil
}

func createCatalogFromWorkingSet(ctx context.Context, dao db.DAO, workingSetID string) (Catalog, error) {
	dbWorkingSet, err := dao.GetWorkingSet(ctx, workingSetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Catalog{}, fmt.Errorf("working set %s not found", workingSetID)
		}
		return Catalog{}, fmt.Errorf("failed to get working set: %w", err)
	}

	workingSet := workingset.NewFromDb(dbWorkingSet)

	servers := make([]Server, len(workingSet.Servers))
	for i, server := range workingSet.Servers {
		servers[i] = Server{
			Type:     server.Type,
			Tools:    server.Tools,
			Source:   server.Source,
			Image:    server.Image,
			Snapshot: server.Snapshot,
		}
	}

	return Catalog{
		Name:    workingSet.Name,
		Servers: servers,
		Source:  SourcePrefixWorkingSet + workingSet.ID,
	}, nil
}

func createCatalogFromLegacyCatalog(ctx context.Context, legacyCatalogURL string) (Catalog, error) {
	legacyCatalog, name, err := legacycatalog.ReadOne(ctx, legacyCatalogURL)
	if err != nil {
		return Catalog{}, fmt.Errorf("failed to read legacy catalog: %w", err)
	}

	servers := make([]Server, 0, len(legacyCatalog.Servers))
	for name, server := range legacyCatalog.Servers {
		// TODO(cody): Add support for remote servers from the legacy catalog
		if server.Type == "server" && server.Image != "" {
			s := Server{
				Type:  workingset.ServerTypeImage,
				Image: server.Image,
				Snapshot: &workingset.ServerSnapshot{
					Server: server,
				},
			}
			s.Snapshot.Server.Name = name
			servers = append(servers, s)
		}
	}

	slices.SortStableFunc(servers, func(a, b Server) int {
		return strings.Compare(a.Snapshot.Server.Name, b.Snapshot.Server.Name)
	})

	return Catalog{
		Name:    "Legacy Catalog",
		Servers: servers,
		Source:  SourcePrefixLegacyCatalog + name,
	}, nil
}
