package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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

	if err := workingSet.EnsureSnapshotsResolved(ctx, ociService); err != nil {
		return fmt.Errorf("failed to resolve snapshots: %w", err)
	}

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

func RemoveServers(ctx context.Context, dao db.DAO, id string, serverRefs []string) error {
	if len(serverRefs) == 0 {
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

	// Build a set of servers to remove (strip protocol scheme for comparison)
	refsToRemove := make(map[string]bool)
	for _, ref := range serverRefs {
		normalized := stripProtocol(ref)
		refsToRemove[normalized] = true
	}

	// Filter out the servers to remove
	originalCount := len(workingSet.Servers)
	filtered := make([]Server, 0, len(workingSet.Servers))
	for _, server := range workingSet.Servers {
		shouldKeep := true

		switch server.Type {
		case ServerTypeImage:
			if refsToRemove[stripProtocol(server.Image)] {
				shouldKeep = false
			}
		case ServerTypeRegistry:
			if refsToRemove[stripProtocol(server.Source)] {
				shouldKeep = false
			}
		}

		if shouldKeep {
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

// stripProtocol removes the protocol scheme (everything before and including "://") from a URI
func stripProtocol(uri string) string {
	if idx := strings.Index(uri, "://"); idx != -1 {
		return uri[idx+3:]
	}
	return uri
}
