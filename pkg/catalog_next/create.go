package catalognext

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func CreateFromWorkingSet(ctx context.Context, dao db.DAO, workingSetId string, name string) error {
	dbWorkingSet, err := dao.GetWorkingSet(ctx, workingSetId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("working set %s not found", workingSetId)
		}
		return fmt.Errorf("failed to get working set: %w", err)
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

	catalogName := workingSet.Name
	if name != "" {
		catalogName = name
	}

	catalog := Catalog{
		Name:    catalogName,
		Servers: servers,
		Source:  "working-set:" + workingSetId,
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

	fmt.Printf("Catalog %s created with digest %s\n", catalog.Name, catalog.Digest())

	return nil
}
