package catalognext

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

// apiRegistries maps registry hostnames to their API base URLs.
// Registries in this map are pulled via their REST API instead of OCI.
var apiRegistries = map[string]string{
	"registry.modelcontextprotocol.io": registryapi.CommunityRegistryBaseURL,
}

// IsAPIRegistry checks if a reference corresponds to a registry pulled via REST API
func IsAPIRegistry(refStr string) bool {
	name := refStr
	if idx := strings.Index(refStr, ":"); idx != -1 {
		name = refStr[:idx]
	}
	_, ok := apiRegistries[name]
	return ok
}

// GetRegistryURL returns the API base URL for a registry
func GetRegistryURL(refStr string) string {
	name := refStr
	if idx := strings.Index(refStr, ":"); idx != -1 {
		name = refStr[:idx]
	}
	return apiRegistries[name]
}

// PullCommunityOptions contains options for pulling from community registries
type PullCommunityOptions struct {
	// Reserved for future options
}

// DefaultPullCommunityOptions returns the default options
func DefaultPullCommunityOptions() PullCommunityOptions {
	return PullCommunityOptions{}
}

// PullCommunityResult contains the results of a community registry pull
type PullCommunityResult struct {
	ServersAdded   int
	ServersOCI     int
	ServersRemote  int
	ServersSkipped int
	TotalServers   int
	// SkippedByType tracks skipped server counts by their primary package type
	SkippedByType map[string]int
}

// PullCommunity fetches servers from a community registry API and writes to the database
func PullCommunity(ctx context.Context, dao db.DAO, refStr string, _ PullCommunityOptions) (*PullCommunityResult, error) {
	registryURL := GetRegistryURL(refStr)
	if registryURL == "" {
		return nil, fmt.Errorf("unknown community registry: %s", refStr)
	}

	client := registryapi.NewClient()

	// Fetch all servers from community registry (uses cache if available)
	servers, err := client.ListServers(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch servers from community registry: %w", err)
	}

	// Convert to catalog format (API already returns only latest versions)
	catalogServers := make(map[string]catalog.Server)
	skippedByType := make(map[string]int)
	var ociCount, remoteCount int

	for _, serverResp := range servers {
		catalogServer, err := catalog.TransformToDocker(serverResp.Server)
		if err != nil {
			// Categorize skipped server by its primary package type
			if len(serverResp.Server.Packages) > 0 {
				skippedByType[serverResp.Server.Packages[0].RegistryType]++
			} else {
				skippedByType["none"]++
			}
			continue
		}

		switch catalogServer.Type {
		case "server":
			ociCount++
		case "remote":
			remoteCount++
		}

		// Add "community" tag to identify source
		if catalogServer.Metadata == nil {
			catalogServer.Metadata = &catalog.Metadata{}
		}
		catalogServer.Metadata.Tags = appendIfMissing(catalogServer.Metadata.Tags, "community")

		catalogServers[catalogServer.Name] = *catalogServer
	}

	// Normalize ref with :latest tag for OCI parser compatibility
	catalogRef := refStr
	if !strings.Contains(refStr, ":") {
		catalogRef = refStr + ":latest"
	}

	// Write to database
	if err := writeCommunityToDatabase(ctx, dao, catalogRef, catalogServers); err != nil {
		return nil, fmt.Errorf("failed to write catalog to database: %w", err)
	}

	skippedTotal := 0
	for _, count := range skippedByType {
		skippedTotal += count
	}

	return &PullCommunityResult{
		ServersAdded:   len(catalogServers),
		ServersOCI:     ociCount,
		ServersRemote:  remoteCount,
		ServersSkipped: skippedTotal,
		TotalServers:   len(servers),
		SkippedByType:  skippedByType,
	}, nil
}

// writeCommunityToDatabase writes the community catalog to the database
func writeCommunityToDatabase(ctx context.Context, dao db.DAO, catalogRef string, catalogServers map[string]catalog.Server) error {
	// Convert catalog.Server entries to Server entries with snapshots
	nextServers := make([]Server, 0, len(catalogServers))
	for name, server := range catalogServers {
		// Ensure server has its name set
		server.Name = name

		var nextServer Server
		switch server.Type {
		case "remote":
			nextServer = Server{
				Type:     workingset.ServerTypeRemote,
				Endpoint: server.Remote.URL,
				Snapshot: &workingset.ServerSnapshot{
					Server: server,
				},
			}
		default:
			nextServer = Server{
				Type:  workingset.ServerTypeImage,
				Image: server.Image,
				Snapshot: &workingset.ServerSnapshot{
					Server: server,
				},
			}
		}
		nextServers = append(nextServers, nextServer)
	}

	// Sort servers by name for consistent ordering
	sort.Slice(nextServers, func(i, j int) bool {
		return nextServers[i].Snapshot.Server.Name < nextServers[j].Snapshot.Server.Name
	})

	// Create the catalog structure
	nextCatalog := Catalog{
		Ref:    catalogRef,
		Source: SourcePrefixRegistry + catalogRef,
		CatalogArtifact: CatalogArtifact{
			Title:   "MCP Community Registry",
			Servers: nextServers,
		},
	}

	// Convert to database format and upsert
	dbCatalog, err := nextCatalog.ToDb()
	if err != nil {
		return fmt.Errorf("failed to convert catalog to database format: %w", err)
	}

	if err := dao.UpsertCatalog(ctx, dbCatalog); err != nil {
		return fmt.Errorf("failed to upsert catalog: %w", err)
	}

	return nil
}

// appendIfMissing appends a value to a slice if it's not already present
func appendIfMissing(slice []string, val string) []string {
	for _, item := range slice {
		if item == val {
			return slice
		}
	}
	return append(slice, val)
}
