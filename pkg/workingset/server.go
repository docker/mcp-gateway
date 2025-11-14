package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/pkg/sliceutil"
)

func AddServers(ctx context.Context, dao db.DAO, registryClient registryapi.Client, ociService oci.Service, id string, servers []string, catalogRef string, catalogServers []string) error {
	if len(servers) == 0 && len(catalogServers) == 0 {
		return fmt.Errorf("at least one server must be specified")
	}
	if len(catalogServers) > 0 && catalogRef == "" {
		return fmt.Errorf("catalog must be specified when adding catalog servers")
	}

	dbWorkingSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("working set %s not found", id)
		}
		return fmt.Errorf("failed to get working set: %w", err)
	}

	workingSet := NewFromDb(dbWorkingSet)

	defaultSecret := "default"
	_, defaultFound := workingSet.Secrets[defaultSecret]
	if workingSet.Secrets == nil || !defaultFound {
		defaultSecret = ""
	}

	// Handle direct server references
	newServers := make([]Server, len(servers))
	for i, server := range servers {
		s, err := resolveServerFromString(ctx, registryClient, ociService, server)
		if err != nil {
			return fmt.Errorf("invalid server value: %w", err)
		}
		newServers[i] = s
	}

	workingSet.Servers = append(workingSet.Servers, newServers...)

	// Handle catalog server references
	if catalogRef != "" && len(catalogServers) > 0 {
		ref, err := name.ParseReference(catalogRef)
		if err != nil {
			return fmt.Errorf("failed to parse catalog reference %s: %w", catalogRef, err)
		}
		catalogRef = oci.FullNameWithoutDigest(ref)

		dbCatalog, err := dao.GetCatalog(ctx, catalogRef)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("catalog %s not found", catalogRef)
			}
			return fmt.Errorf("failed to get catalog: %w", err)
		}
		filteredServers := make([]db.CatalogServer, 0, len(dbCatalog.Servers))
		for _, server := range dbCatalog.Servers {
			if slices.Contains(catalogServers, server.Snapshot.Server.Name) {
				filteredServers = append(filteredServers, server)
			}
		}
		if len(filteredServers) != len(catalogServers) {
			missingServers := sliceutil.Difference(catalogServers, sliceutil.Map(filteredServers, func(server db.CatalogServer) string { return server.Snapshot.Server.Name }))
			return fmt.Errorf("servers were not found in catalog: %v", missingServers)
		}
		catalogServers := mapCatalogServersToWorkingSetServers(filteredServers, defaultSecret)
		workingSet.Servers = append(workingSet.Servers, catalogServers...)
	}

	if err := workingSet.Validate(); err != nil {
		return fmt.Errorf("invalid working set: %w", err)
	}

	err = dao.UpdateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to update working set: %w", err)
	}

	fmt.Printf("Added %d server(s) to working set %s\n", len(newServers)+len(catalogServers), id)

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

func mapCatalogServersToWorkingSetServers(dbServers []db.CatalogServer, defaultSecret string) []Server {
	servers := make([]Server, len(dbServers))
	for i, server := range dbServers {
		servers[i] = Server{
			Type:   ServerType(server.ServerType),
			Tools:  server.Tools,
			Config: map[string]any{},
			Source: server.Source,
			Image:  server.Image,
			Snapshot: &ServerSnapshot{
				Server: server.Snapshot.Server,
			},
			Secrets: defaultSecret,
		}
	}
	return servers
}
