package workingset

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/formatting"
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
			return fmt.Errorf("profile %s not found", id)
		}
		return fmt.Errorf("failed to get profile: %w", err)
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
		return fmt.Errorf("invalid profile: %w", err)
	}

	err = dao.UpdateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	fmt.Printf("Added %d server(s) to profile %s\n", len(newServers)+len(catalogServers), id)

	return nil
}

func RemoveServers(ctx context.Context, dao db.DAO, id string, serverNames []string) error {
	if len(serverNames) == 0 {
		return fmt.Errorf("at least one server must be specified")
	}

	dbWorkingSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("profile %s not found", id)
		}
		return fmt.Errorf("failed to get profile: %w", err)
	}

	workingSet := NewFromDb(dbWorkingSet)

	namesToRemove := make(map[string]bool)
	for _, name := range serverNames {
		namesToRemove[name] = true
	}

	originalCount := len(workingSet.Servers)
	filtered := make([]Server, 0, len(workingSet.Servers))
	for _, server := range workingSet.Servers {
		if !namesToRemove[server.Snapshot.Server.Name] {
			filtered = append(filtered, server)
		}
	}

	removedCount := originalCount - len(filtered)
	if removedCount == 0 {
		return fmt.Errorf("no matching servers found to remove")
	}

	workingSet.Servers = filtered

	if err := workingSet.Validate(); err != nil {
		return fmt.Errorf("invalid profile: %w", err)
	}

	err = dao.UpdateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	fmt.Printf("Removed %d server(s) from profile %s\n", removedCount, id)

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

type SearchResult struct {
	ID      string   `json:"id" yaml:"id"`
	Name    string   `json:"name" yaml:"name"`
	Servers []Server `json:"servers" yaml:"servers"`
}

type serverFilter struct {
	key   string
	value string
}

func ListServers(ctx context.Context, dao db.DAO, filters []string, format OutputFormat) error {
	parsedFilters, err := parseFilters(filters)
	if err != nil {
		return err
	}

	var nameFilter string
	var workingSetFilter string
	for _, filter := range parsedFilters {
		switch filter.key {
		case "name":
			nameFilter = filter.value
		case "profile":
			workingSetFilter = filter.value
		default:
			return fmt.Errorf("unsupported filter key: %s", filter.key)
		}
	}
	dbSets, err := dao.SearchWorkingSets(ctx, "", workingSetFilter)
	if err != nil {
		return fmt.Errorf("failed to search profiles: %w", err)
	}
	results := buildSearchResults(dbSets, nameFilter)
	return outputSearchResults(results, format)
}

func parseFilters(filters []string) ([]serverFilter, error) {
	parsed := make([]serverFilter, 0, len(filters))
	for _, filter := range filters {
		parts := strings.SplitN(filter, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid filter format: %s (expected key=value)", filter)
		}
		parsed = append(parsed, serverFilter{
			key:   parts[0],
			value: parts[1],
		})
	}
	return parsed, nil
}

func buildSearchResults(dbSets []db.WorkingSet, nameFilter string) []SearchResult {
	nameLower := strings.ToLower(nameFilter)
	results := make([]SearchResult, 0, len(dbSets))

	for _, dbSet := range dbSets {
		workingSet := NewFromDb(&dbSet)
		matchedServers := make([]Server, 0)

		for _, server := range workingSet.Servers {
			if matchesNameFilter(server, nameLower) {
				matchedServers = append(matchedServers, server)
			}
		}
		if len(matchedServers) == 0 {
			continue
		}
		sort.Slice(matchedServers, func(i, j int) bool {
			return matchedServers[i].Snapshot.Server.Name < matchedServers[j].Snapshot.Server.Name
		})
		results = append(results, SearchResult{
			ID:      workingSet.ID,
			Name:    workingSet.Name,
			Servers: matchedServers,
		})
	}
	return results
}

func matchesNameFilter(server Server, nameLower string) bool {
	if nameLower == "" {
		return true
	}
	serverName := strings.ToLower(server.Snapshot.Server.Name)
	return strings.Contains(serverName, nameLower)
}

func outputSearchResults(results []SearchResult, format OutputFormat) error {
	var data []byte
	var err error

	switch format {
	case OutputFormatHumanReadable:
		printSearchResultsHuman(results)
		return nil
	case OutputFormatJSON:
		data, err = json.MarshalIndent(results, "", "  ")
	case OutputFormatYAML:
		data, err = yaml.Marshal(results)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to format search results: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func printSearchResultsHuman(results []SearchResult) {
	if len(results) == 0 {
		fmt.Println("No profiles found")
		return
	}

	rows := [][]string{}

	for _, result := range results {
		for _, server := range result.Servers {
			rows = append(rows, []string{
				result.ID,
				string(server.Type),
				server.Snapshot.Server.Name,
			})
		}
	}

	header := []string{"PROFILE", "TYPE", "IDENTIFIER"}
	formatting.PrettyPrintTable(rows, []int{40, 10, 120}, header)
}
