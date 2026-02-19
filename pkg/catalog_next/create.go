package catalognext

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"

	legacycatalog "github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/pkg/telemetry"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func Create(ctx context.Context, dao db.DAO, registryClient registryapi.Client, ociService oci.Service, refStr string, servers []string, workingSetID string, legacyCatalogURL string, communityRegistryRef string, title string) error {
	telemetry.Init()
	start := time.Now()
	var success bool
	defer func() {
		duration := time.Since(start)
		telemetry.RecordCatalogOperation(ctx, "create", refStr, float64(duration.Milliseconds()), success)
	}()
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse oci-reference %s: %w", refStr, err)
	}
	if !oci.IsValidInputReference(ref) {
		return fmt.Errorf("reference must be a valid OCI reference without a digest")
	}

	var catalog Catalog
	if workingSetID != "" {
		catalog, err = createCatalogFromWorkingSet(ctx, dao, workingSetID)
		if err != nil {
			return fmt.Errorf("failed to create catalog from profile: %w", err)
		}
	} else if legacyCatalogURL != "" {
		catalog, err = createCatalogFromLegacyCatalog(ctx, legacyCatalogURL)
		if err != nil {
			return fmt.Errorf("failed to create catalog from legacy catalog: %w", err)
		}
	} else if communityRegistryRef != "" {
		catalog, err = createCatalogFromCommunityRegistry(ctx, registryClient, communityRegistryRef)
		if err != nil {
			return fmt.Errorf("failed to create catalog from community registry: %w", err)
		}
	} else {
		// Construct from servers
		if title == "" {
			return fmt.Errorf("title is required when creating a catalog without using an existing legacy catalog, profile, or community registry")
		}
		catalog = Catalog{
			CatalogArtifact: CatalogArtifact{
				Title:   title,
				Servers: make([]Server, 0, len(servers)),
			},
			Source: SourcePrefixUser + "cli",
		}
	}

	catalog.Ref = oci.FullNameWithoutDigest(ref)

	if title != "" {
		catalog.Title = title
	}

	if err := addServersToCatalog(ctx, dao, registryClient, ociService, &catalog, servers); err != nil {
		return err
	}

	if err := catalog.Validate(); err != nil {
		return fmt.Errorf("invalid catalog: %w", err)
	}

	dbCatalog, err := catalog.ToDb()
	if err != nil {
		return fmt.Errorf("failed to convert catalog to db: %w", err)
	}

	err = dao.UpsertCatalog(ctx, dbCatalog)
	if err != nil {
		return fmt.Errorf("failed to create catalog: %w", err)
	}

	fmt.Printf("Catalog %s created\n", catalog.Ref)

	success = true
	return nil
}

func addServersToCatalog(ctx context.Context, dao db.DAO, registryClient registryapi.Client, ociService oci.Service, catalog *Catalog, servers []string) error {
	if len(servers) == 0 {
		return nil
	}

	for _, server := range servers {
		ss, err := workingset.ResolveServersFromString(ctx, registryClient, ociService, dao, server)
		if err != nil {
			return err
		}
		for _, s := range ss {
			catalog.Servers = append(catalog.Servers, workingSetServerToCatalogServer(s))
		}
	}

	return nil
}

func createCatalogFromWorkingSet(ctx context.Context, dao db.DAO, workingSetID string) (Catalog, error) {
	dbWorkingSet, err := dao.GetWorkingSet(ctx, workingSetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Catalog{}, fmt.Errorf("profile %s not found", workingSetID)
		}
		return Catalog{}, fmt.Errorf("failed to get profile: %w", err)
	}

	workingSet := workingset.NewFromDb(dbWorkingSet)

	servers := make([]Server, len(workingSet.Servers))
	for i, server := range workingSet.Servers {
		servers[i] = workingSetServerToCatalogServer(server)
	}

	return Catalog{
		CatalogArtifact: CatalogArtifact{
			Title:   workingSet.Name,
			Servers: servers,
		},
		Source: SourcePrefixWorkingSet + workingSet.ID,
	}, nil
}

func createCatalogFromLegacyCatalog(ctx context.Context, legacyCatalogURL string) (Catalog, error) {
	legacyCatalog, name, displayName, err := legacycatalog.ReadOne(ctx, legacyCatalogURL)
	if err != nil {
		return Catalog{}, fmt.Errorf("failed to read legacy catalog: %w", err)
	}

	servers := make([]Server, 0, len(legacyCatalog.Servers))
	for name, server := range legacyCatalog.Servers {
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
		} else if server.Type == "remote" {
			s := Server{
				Type:     workingset.ServerTypeRemote,
				Endpoint: server.Remote.URL,
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

	if displayName == "" {
		displayName = "Legacy Catalog"
	}

	return Catalog{
		CatalogArtifact: CatalogArtifact{
			Title:   displayName,
			Servers: servers,
		},
		Source: SourcePrefixLegacyCatalog + name,
	}, nil
}

func workingSetServerToCatalogServer(server workingset.Server) Server {
	return Server{
		Type:     server.Type,
		Tools:    server.Tools,
		Source:   server.Source,
		Image:    server.Image,
		Endpoint: server.Endpoint,
		Snapshot: server.Snapshot,
	}
}

type communityRegistryResult struct {
	serversAdded   int
	serversOCI     int
	serversRemote  int
	serversSkipped int
	totalServers   int
	skippedByType  map[string]int
}

func createCatalogFromCommunityRegistry(ctx context.Context, registryClient registryapi.Client, registryRef string) (Catalog, error) {
	baseURL := "https://" + registryRef
	servers, err := registryClient.ListServers(ctx, baseURL, "")
	if err != nil {
		return Catalog{}, fmt.Errorf("failed to fetch servers from community registry: %w", err)
	}

	catalogServers := make([]Server, 0)
	skippedByType := make(map[string]int)
	var ociCount, remoteCount int

	for _, serverResp := range servers {
		catalogServer, err := legacycatalog.TransformToDocker(ctx, serverResp.Server, legacycatalog.WithAllowPyPI(false))
		if err != nil {
			if !errors.Is(err, legacycatalog.ErrIncompatibleServer) {
				fmt.Fprintf(os.Stderr, "Warning: failed to transform server %q: %v\n", serverResp.Server.Name, err)
			}
			if len(serverResp.Server.Packages) > 0 {
				skippedByType[serverResp.Server.Packages[0].RegistryType]++
			} else {
				skippedByType["none"]++
			}
			continue
		}

		// Tag with "community" for source identification
		if catalogServer.Metadata == nil {
			catalogServer.Metadata = &legacycatalog.Metadata{}
		}
		catalogServer.Metadata.Tags = appendIfMissing(catalogServer.Metadata.Tags, "community")

		var s Server
		switch catalogServer.Type {
		case "server":
			ociCount++
			s = Server{
				Type:  workingset.ServerTypeImage,
				Image: catalogServer.Image,
				Snapshot: &workingset.ServerSnapshot{
					Server: *catalogServer,
				},
			}
		case "remote":
			remoteCount++
			s = Server{
				Type:     workingset.ServerTypeRemote,
				Endpoint: catalogServer.Remote.URL,
				Snapshot: &workingset.ServerSnapshot{
					Server: *catalogServer,
				},
			}
		default:
			continue
		}
		catalogServers = append(catalogServers, s)
	}

	slices.SortStableFunc(catalogServers, func(a, b Server) int {
		return strings.Compare(a.Snapshot.Server.Name, b.Snapshot.Server.Name)
	})

	result := communityRegistryResult{
		serversAdded:   len(catalogServers),
		serversOCI:     ociCount,
		serversRemote:  remoteCount,
		serversSkipped: totalSkipped(skippedByType),
		totalServers:   len(servers),
		skippedByType:  skippedByType,
	}
	printCommunityRegistryResult(registryRef, result)

	return Catalog{
		CatalogArtifact: CatalogArtifact{
			Title:   "MCP Community Registry",
			Servers: catalogServers,
		},
		Source: SourcePrefixRegistry + registryRef,
	}, nil
}

func totalSkipped(skippedByType map[string]int) int {
	total := 0
	for _, count := range skippedByType {
		total += count
	}
	return total
}

func printCommunityRegistryResult(refStr string, result communityRegistryResult) {
	fmt.Fprintf(os.Stderr, "Fetched %d servers from %s\n", result.serversAdded, refStr)
	fmt.Fprintf(os.Stderr, "  Total in registry: %d\n", result.totalServers)
	fmt.Fprintf(os.Stderr, "  Imported:          %d\n", result.serversAdded)
	fmt.Fprintf(os.Stderr, "    OCI (stdio):     %d\n", result.serversOCI)
	fmt.Fprintf(os.Stderr, "    Remote:          %d\n", result.serversRemote)
	fmt.Fprintf(os.Stderr, "  Skipped:           %d\n", result.serversSkipped)

	if len(result.skippedByType) > 0 {
		type typeCount struct {
			name  string
			count int
		}
		var sorted []typeCount
		for t, c := range result.skippedByType {
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
			fmt.Fprintf(os.Stderr, "    %-17s%d\n", label+":", tc.count)
		}
	}
}

func appendIfMissing(slice []string, val string) []string {
	for _, item := range slice {
		if item == val {
			return slice
		}
	}
	return append(slice, val)
}
