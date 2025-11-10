package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
)

func AddServers(ctx context.Context, dao db.DAO, registryClient registryapi.Client, ociService oci.Service, id string, servers []string) error {
	if len(servers) == 0 {
		return fmt.Errorf("at least one server must be specified")
	}

	dbWorkingSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("working set %s not found", id)
		}
		return fmt.Errorf("failed to get working set: %w", err)
	}

	workingSet := NewFromDb(dbWorkingSet)

	newServers := make([]Server, len(servers))
	for i, server := range servers {
		s, err := resolveServerFromString(ctx, registryClient, ociService, server)
		if err != nil {
			return fmt.Errorf("invalid server value: %w", err)
		}
		newServers[i] = s
	}

	workingSet.Servers = append(workingSet.Servers, newServers...)

	if err := workingSet.Validate(); err != nil {
		return fmt.Errorf("invalid working set: %w", err)
	}

	err = dao.UpdateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to update working set: %w", err)
	}

	fmt.Printf("Added %d server(s) to working set %s\n", len(newServers), id)

	return nil
}

func RemoveServers(ctx context.Context, dao db.DAO, id string, serverNames []string) error {
	if len(serverNames) == 0 {
		return fmt.Errorf("at least one server must be specified")
	}

	dbWorkingSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("working set %s not found", id)
		}
		return fmt.Errorf("failed to get working set: %w", err)
	}

	workingSet := NewFromDb(dbWorkingSet)

	namesToRemove := make(map[string]bool)
	for _, name := range serverNames {
		namesToRemove[name] = true
	}

	originalCount := len(workingSet.Servers)
	filtered := make([]Server, 0, len(workingSet.Servers))
	for _, server := range workingSet.Servers {
		// TODO: Remove when Snapshot is required
		if server.Snapshot == nil || !namesToRemove[server.Snapshot.Server.Name] {
			filtered = append(filtered, server)
		}
	}

	removedCount := originalCount - len(filtered)
	if removedCount == 0 {
		return fmt.Errorf("no matching servers found to remove")
	}

	workingSet.Servers = filtered

	if err := workingSet.Validate(); err != nil {
		return fmt.Errorf("invalid working set: %w", err)
	}

	err = dao.UpdateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to update working set: %w", err)
	}

	fmt.Printf("Removed %d server(s) from working set %s\n", removedCount, id)

	return nil
}
