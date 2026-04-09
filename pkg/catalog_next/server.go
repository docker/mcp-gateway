package catalognext

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/google/go-containerregistry/pkg/name"
	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/fetch"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/policy"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

type InspectResult struct {
	Server        `yaml:",inline"`
	ReadmeContent string `json:"readmeContent,omitempty" yaml:"readmeContent,omitempty"`
}

type serverFilter struct {
	key   string
	value string
}

func InspectServer(ctx context.Context, dao db.DAO, registryClient registryapi.Client, catalogRef string, serverName string, format workingset.OutputFormat) error {
	ref, err := name.ParseReference(catalogRef)
	if err != nil {
		return fmt.Errorf("failed to parse oci-reference %s: %w", catalogRef, err)
	}
	if !oci.IsValidInputReference(ref) {
		return fmt.Errorf("reference %s must be a valid OCI reference without a digest", catalogRef)
	}

	catalogRef = oci.FullNameWithoutDigest(ref)

	// Get the catalog
	dbCatalog, err := dao.GetCatalog(ctx, catalogRef)
	if err != nil {
		return fmt.Errorf("failed to get catalog %s: %w", catalogRef, err)
	}

	catalog := NewFromDb(dbCatalog)

	server := catalog.FindServer(serverName)
	if server == nil {
		return fmt.Errorf("server %s not found in catalog %s", serverName, catalogRef)
	}

	inspectResult := InspectResult{
		Server: *server,
	}

	if server.Snapshot != nil && server.Snapshot.Server.ReadmeURL != "" {
		readmeContent, err := fetch.Untrusted(ctx, server.Snapshot.Server.ReadmeURL)
		if err != nil {
			// Log but don't fail: the URL may point to a private repo or
			// a path that doesn't exist (e.g. community registry servers
			// whose GitHub README URL was derived from the repository field).
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch readme for %s: %v\n", serverName, err)
		} else {
			inspectResult.ReadmeContent = string(readmeContent)
		}
	}

	// Try live registry API lookup to discover README URL when the snapshot
	// has no baked-in ReadmeURL (common for older community catalogs).
	var registryResp *v0.ServerResponse
	if inspectResult.ReadmeContent == "" && registryClient != nil && server.Snapshot != nil {
		content, resp := fetchReadmeViaRegistryAPI(ctx, registryClient, &server.Snapshot.Server)
		if content != "" {
			inspectResult.ReadmeContent = content
		}
		registryResp = resp
	}

	// When no README content is available, synthesize an overview from the
	// server's metadata so the overview tab is not empty.
	if inspectResult.ReadmeContent == "" && server.Snapshot != nil {
		inspectResult.ReadmeContent = buildSynthesizedOverview(&server.Snapshot.Server, registryResp)
	}

	var data []byte

	switch format {
	case workingset.OutputFormatJSON:
		data, err = json.MarshalIndent(inspectResult, "", "  ")
	case workingset.OutputFormatYAML, workingset.OutputFormatHumanReadable:
		data, err = yaml.Marshal(inspectResult)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal server: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

// buildSynthesizedOverview constructs a markdown overview from a server's
// catalog metadata. This is used as a fallback when the server has no
// fetchable README (common for community registry servers with private
// repos or no repo at all).
//
// When a registry API response is available, its title, status, and
// websiteUrl fields are included to enrich the overview.
//
// The server's description is intentionally omitted because the UI header
// already displays it. It is only used as a last resort when no other
// content can be synthesized at all.
func buildSynthesizedOverview(s *catalog.Server, registryResp *v0.ServerResponse) string {
	if s == nil {
		return ""
	}

	var b strings.Builder

	// Title -- prefer registry API response, fall back to catalog snapshot
	title := s.Title
	if registryResp != nil && registryResp.Server.Title != "" {
		title = registryResp.Server.Title
	}
	if title != "" {
		b.WriteString(fmt.Sprintf("# %s\n\n", title))
	}

	// Status from registry API metadata
	if registryResp != nil && registryResp.Meta.Official != nil {
		status := string(registryResp.Meta.Official.Status)
		if status != "" {
			b.WriteString(fmt.Sprintf("**Status:** %s\n\n", status))
		}
	}

	// Connection info
	if s.Remote.URL != "" {
		b.WriteString("**Remote MCP server**")
		if s.Remote.Transport != "" {
			b.WriteString(fmt.Sprintf(" (%s)", s.Remote.Transport))
		}
		b.WriteString("\n")
	} else if s.Image != "" {
		b.WriteString(fmt.Sprintf("**Runs in Docker container** `%s`\n", s.Image))
	}

	// Tools section
	if len(s.Tools) > 0 {
		b.WriteString("\n## Tools\n\n")
		b.WriteString("| Tool | Description |\n")
		b.WriteString("|------|-------------|\n")
		for _, tool := range s.Tools {
			desc := tool.Description
			if desc == "" {
				desc = "-"
			}
			b.WriteString(fmt.Sprintf("| %s | %s |\n", tool.Name, desc))
		}
	}

	// Configuration section -- show non-secret config schema properties
	if len(s.Config) > 0 {
		if items := extractConfigProperties(s.Config); len(items) > 0 {
			b.WriteString("\n## Configuration\n\n")
			for _, item := range items {
				b.WriteString(item)
				b.WriteString("\n")
			}
		}
	}

	// Authentication section
	if len(s.Secrets) > 0 || s.IsOAuthServer() {
		b.WriteString("\n## Authentication\n\n")
		if s.IsOAuthServer() {
			for _, provider := range s.OAuth.Providers {
				b.WriteString(fmt.Sprintf("- OAuth provider: **%s**\n", provider.Provider))
			}
		}
		for _, secret := range s.Secrets {
			// Show the environment variable name (more useful than the
			// fully-qualified internal secret key).
			if secret.Env != "" {
				b.WriteString(fmt.Sprintf("- `%s`\n", secret.Env))
			} else {
				b.WriteString(fmt.Sprintf("- `%s`\n", secret.Name))
			}
		}
	}

	// Metadata section
	if s.Metadata != nil {
		var metaItems []string
		if s.Metadata.Category != "" {
			metaItems = append(metaItems, fmt.Sprintf("- **Category:** %s", s.Metadata.Category))
		}
		if s.Metadata.License != "" {
			metaItems = append(metaItems, fmt.Sprintf("- **License:** %s", s.Metadata.License))
		}
		if len(s.Metadata.Tags) > 0 {
			metaItems = append(metaItems, fmt.Sprintf("- **Tags:** %s", strings.Join(s.Metadata.Tags, ", ")))
		}
		if len(metaItems) > 0 {
			b.WriteString("\n## Details\n\n")
			for _, item := range metaItems {
				b.WriteString(item)
				b.WriteString("\n")
			}
		}
	}

	// Links section
	var links []string
	if registryResp != nil && registryResp.Server.WebsiteURL != "" {
		links = append(links, fmt.Sprintf("- [Website](%s)", registryResp.Server.WebsiteURL))
	}
	if s.Metadata != nil && s.Metadata.RegistryURL != "" {
		links = append(links, fmt.Sprintf("- [MCP Registry](%s)", s.Metadata.RegistryURL))
	}
	if s.Remote.URL != "" {
		links = append(links, fmt.Sprintf("- Endpoint: `%s`", s.Remote.URL))
	}
	if s.ReadmeURL != "" {
		links = append(links, fmt.Sprintf("- [Source Repository](%s)", sourceRepoFromReadmeURL(s.ReadmeURL)))
	}
	if len(links) > 0 {
		b.WriteString("\n## Links\n\n")
		for _, link := range links {
			b.WriteString(link)
			b.WriteString("\n")
		}
	}

	// If we produced no structured content at all, fall back to the
	// description so the overview is not completely blank.
	if b.Len() == 0 && s.Description != "" {
		return s.Description + "\n"
	}

	return b.String()
}

// extractConfigProperties pulls human-readable property descriptions from
// the Config []any slice. Each element is expected to be a JSON-schema-like
// map with a "properties" key.
func extractConfigProperties(config []any) []string {
	var items []string
	for _, cfg := range config {
		configMap, ok := cfg.(map[string]any)
		if !ok {
			continue
		}
		props, ok := configMap["properties"].(map[string]any)
		if !ok {
			continue
		}
		for propName, propVal := range props {
			propMap, ok := propVal.(map[string]any)
			if !ok {
				continue
			}
			desc, _ := propMap["description"].(string)
			if desc != "" {
				items = append(items, fmt.Sprintf("- `%s`: %s", propName, desc))
			} else {
				items = append(items, fmt.Sprintf("- `%s`", propName))
			}
		}
	}
	return items
}

// sourceRepoFromReadmeURL extracts a GitHub repository URL from a
// raw.githubusercontent.com README URL. Returns the input unchanged
// if it does not match the expected pattern.
func sourceRepoFromReadmeURL(readmeURL string) string {
	const prefix = "https://raw.githubusercontent.com/"
	if !strings.HasPrefix(readmeURL, prefix) {
		return readmeURL
	}
	rest := strings.TrimPrefix(readmeURL, prefix)
	parts := strings.SplitN(rest, "/", 3) // owner/repo/...
	if len(parts) < 2 {
		return readmeURL
	}
	return fmt.Sprintf("https://github.com/%s/%s", parts[0], parts[1])
}

// fetchReadmeViaRegistryAPI attempts to discover a README by calling the
// community registry API using the server's Metadata.RegistryURL. It extracts
// the repository info from the API response, derives a GitHub raw README URL,
// and fetches the content. Returns the README content (empty on failure) and
// the registry API response (nil if the API call itself failed). The response
// is returned even when the README fetch fails so callers can use its metadata
// (title, status, websiteUrl) for the synthesized overview.
func fetchReadmeViaRegistryAPI(ctx context.Context, client registryapi.Client, s *catalog.Server) (string, *v0.ServerResponse) {
	if s.Metadata == nil || s.Metadata.RegistryURL == "" {
		return "", nil
	}

	serverURL, err := registryapi.ParseServerURL(s.Metadata.RegistryURL)
	if err != nil {
		return "", nil
	}

	resp, err := client.GetServer(ctx, serverURL)
	if err != nil {
		return "", nil
	}

	if resp.Server.Repository.URL == "" {
		return "", &resp
	}

	readmeURL := catalog.BuildGitHubReadmeURL(resp.Server.Repository.URL, resp.Server.Repository.Subfolder)
	if readmeURL == "" {
		return "", &resp
	}

	content, err := fetch.Untrusted(ctx, readmeURL)
	if err == nil {
		return string(content), &resp
	}

	// The raw.githubusercontent.com URL failed (commonly a 404 due to
	// README filename casing: readme.md vs README.md, or the repo being
	// private/deleted). Fall back to the GitHub API readme endpoint which
	// auto-discovers the README regardless of filename or casing.
	apiContent, apiErr := catalog.FetchGitHubReadmeViaAPI(ctx, resp.Server.Repository.URL, resp.Server.Repository.Subfolder)
	if apiErr != nil {
		return "", &resp
	}
	return apiContent, &resp
}

// ListServers lists servers in a catalog with optional filtering
func ListServers(ctx context.Context, dao db.DAO, catalogRef string, filters []string, format workingset.OutputFormat) error {
	parsedFilters, err := parseFilters(filters)
	if err != nil {
		return err
	}

	ref, err := name.ParseReference(catalogRef)
	if err != nil {
		return fmt.Errorf("failed to parse oci-reference %s: %w", catalogRef, err)
	}
	if !oci.IsValidInputReference(ref) {
		return fmt.Errorf("reference %s must be a valid OCI reference without a digest", catalogRef)
	}

	catalogRef = oci.FullNameWithoutDigest(ref)

	// Get the catalog
	dbCatalog, err := dao.GetCatalog(ctx, catalogRef)
	if err != nil {
		return fmt.Errorf("failed to get catalog %s: %w", catalogRef, err)
	}

	catalog := NewFromDb(dbCatalog)

	policyClient := policycli.ClientForCLI(ctx)
	showPolicy := policyClient != nil
	attachCatalogPolicy(ctx, policyClient, catalog.Ref, &catalog, true)

	// Apply name filter
	var nameFilter string
	for _, filter := range parsedFilters {
		switch filter.key {
		case "name":
			nameFilter = filter.value
		default:
			return fmt.Errorf("unsupported filter key: %s", filter.key)
		}
	}

	// Filter servers
	servers := filterServers(catalog.Servers, nameFilter)

	// Output results
	return outputServers(catalog.Ref, catalog.Title, catalog.Policy, servers, format, showPolicy)
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

func filterServers(servers []Server, nameFilter string) []Server {
	if nameFilter == "" {
		return servers
	}

	nameLower := strings.ToLower(nameFilter)
	filtered := make([]Server, 0)

	for _, server := range servers {
		if matchesNameFilter(server, nameLower) {
			filtered = append(filtered, server)
		}
	}

	return filtered
}

func matchesNameFilter(server Server, nameLower string) bool {
	if server.Snapshot == nil {
		return false
	}
	serverName := strings.ToLower(server.Snapshot.Server.Name)
	return strings.Contains(serverName, nameLower)
}

func outputServers(catalogRef, catalogTitle string, catalogPolicy *policy.Decision, servers []Server, format workingset.OutputFormat, showPolicy bool) error {
	// Sort servers by name
	sort.Slice(servers, func(i, j int) bool {
		if servers[i].Snapshot == nil || servers[j].Snapshot == nil {
			return false
		}
		return servers[i].Snapshot.Server.Name < servers[j].Snapshot.Server.Name
	})

	var data []byte
	var err error

	switch format {
	case workingset.OutputFormatHumanReadable:
		printServersHuman(catalogRef, catalogTitle, catalogPolicy, servers, showPolicy)
		return nil
	case workingset.OutputFormatJSON:
		output := map[string]any{
			"catalog": catalogRef,
			"title":   catalogTitle,
			"servers": servers,
		}
		if showPolicy && catalogPolicy != nil {
			output["policy"] = catalogPolicy
		}
		data, err = json.MarshalIndent(output, "", "  ")
	case workingset.OutputFormatYAML:
		output := map[string]any{
			"catalog": catalogRef,
			"title":   catalogTitle,
			"servers": servers,
		}
		if showPolicy && catalogPolicy != nil {
			output["policy"] = catalogPolicy
		}
		data, err = yaml.Marshal(output)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to format servers: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func printServersHuman(catalogRef, catalogTitle string, catalogPolicy *policy.Decision, servers []Server, showPolicy bool) {
	if len(servers) == 0 {
		fmt.Println("No servers found")
		return
	}

	fmt.Printf("Catalog: %s\n", catalogRef)
	fmt.Printf("Title: %s\n", catalogTitle)
	if showPolicy {
		fmt.Printf("Policy: %s\n", policycli.StatusMessage(catalogPolicy))
	}
	fmt.Printf("Servers (%d):\n\n", len(servers))

	for _, server := range servers {
		if server.Snapshot == nil {
			continue
		}
		srv := server.Snapshot.Server
		fmt.Printf("  %s\n", srv.Name)
		if srv.Title != "" {
			fmt.Printf("    Title: %s\n", srv.Title)
		}
		if srv.Description != "" {
			fmt.Printf("    Description: %s\n", srv.Description)
		}
		fmt.Printf("    Type: %s\n", server.Type)
		switch server.Type {
		case workingset.ServerTypeImage:
			fmt.Printf("    Image: %s\n", server.Image)
		case workingset.ServerTypeRegistry:
			fmt.Printf("    Source: %s\n", server.Source)
		case workingset.ServerTypeRemote:
			fmt.Printf("    Endpoint: %s\n", server.Endpoint)
		}
		if showPolicy {
			fmt.Printf("    Policy: %s\n", policycli.StatusMessage(server.Policy))
		}
		if len(srv.Tools) > 0 {
			fmt.Printf("    Tools: %d\n", allowedToolCount(srv.Tools))
		}
		fmt.Println()
	}
}

// AddServers adds servers to a catalog using various URI schemes
func AddServers(ctx context.Context, dao db.DAO, registryClient registryapi.Client, ociService oci.Service, catalogRef string, serverRefs []string) error {
	if len(serverRefs) == 0 {
		return fmt.Errorf("at least one server must be specified")
	}

	ref, err := name.ParseReference(catalogRef)
	if err != nil {
		return fmt.Errorf("failed to parse oci-reference %s: %w", catalogRef, err)
	}
	if !oci.IsValidInputReference(ref) {
		return fmt.Errorf("reference %s must be a valid OCI reference without a digest", catalogRef)
	}

	catalogRef = oci.FullNameWithoutDigest(ref)

	// Get the catalog
	dbCatalog, err := dao.GetCatalog(ctx, catalogRef)
	if err != nil {
		return fmt.Errorf("failed to get catalog %s: %w", catalogRef, err)
	}

	catalog := NewFromDb(dbCatalog)

	// Resolve all server references
	allServers := make([]workingset.Server, 0)
	for _, serverRef := range serverRefs {
		servers, err := workingset.ResolveServersFromString(ctx, registryClient, ociService, dao, serverRef)
		if err != nil {
			return fmt.Errorf("failed to resolve server reference %q: %w", serverRef, err)
		}
		allServers = append(allServers, servers...)
	}

	if len(allServers) == 0 {
		return fmt.Errorf("no servers found in provided references")
	}

	// Build set of incoming server names for upsert detection
	newServerNames := make(map[string]bool)
	for _, ws := range allServers {
		if ws.Snapshot != nil {
			newServerNames[ws.Snapshot.Server.Name] = true
		}
	}

	// Remove existing servers that will be replaced (upsert)
	replacedCount := 0
	filtered := make([]Server, 0, len(catalog.Servers))
	for _, existing := range catalog.Servers {
		if existing.Snapshot != nil && newServerNames[existing.Snapshot.Server.Name] {
			fmt.Printf("Replaced server %s in catalog %s\n", existing.Snapshot.Server.Name, catalogRef)
			replacedCount++
		} else {
			filtered = append(filtered, existing)
		}
	}
	catalog.Servers = filtered

	// Convert workingset.Server to catalog Server and append
	addedCount := 0
	for _, wsServer := range allServers {
		if wsServer.Snapshot == nil {
			continue
		}

		catalogServer := Server{
			Type:     wsServer.Type,
			Snapshot: wsServer.Snapshot,
		}

		switch wsServer.Type {
		case workingset.ServerTypeRegistry:
			catalogServer.Source = wsServer.Source
		case workingset.ServerTypeImage:
			catalogServer.Image = wsServer.Image
		case workingset.ServerTypeRemote:
			catalogServer.Endpoint = wsServer.Endpoint
		}

		catalog.Servers = append(catalog.Servers, catalogServer)
		addedCount++
	}

	// Save the updated catalog
	dbCatalogUpdated, err := catalog.ToDb()
	if err != nil {
		return fmt.Errorf("failed to convert catalog to database format: %w", err)
	}

	if err := dao.UpsertCatalog(ctx, dbCatalogUpdated); err != nil {
		return fmt.Errorf("failed to update catalog: %w", err)
	}

	if replacedCount > 0 {
		fmt.Printf("Added %d server(s) to catalog '%s' (replaced %d)\n", addedCount, catalogRef, replacedCount)
	} else {
		fmt.Printf("Added %d server(s) to catalog '%s'\n", addedCount, catalogRef)
	}
	return nil
}

// RemoveServers removes servers from a catalog by name
func RemoveServers(ctx context.Context, dao db.DAO, catalogRef string, serverNames []string) error {
	if len(serverNames) == 0 {
		return fmt.Errorf("at least one server name must be specified")
	}

	ref, err := name.ParseReference(catalogRef)
	if err != nil {
		return fmt.Errorf("failed to parse oci-reference %s: %w", catalogRef, err)
	}
	if !oci.IsValidInputReference(ref) {
		return fmt.Errorf("reference %s must be a valid OCI reference without a digest", catalogRef)
	}

	catalogRef = oci.FullNameWithoutDigest(ref)

	// Get the catalog
	dbCatalog, err := dao.GetCatalog(ctx, catalogRef)
	if err != nil {
		return fmt.Errorf("failed to get catalog %s: %w", catalogRef, err)
	}

	catalog := NewFromDb(dbCatalog)

	// Create a set of names to remove
	namesToRemove := make(map[string]bool)
	for _, name := range serverNames {
		namesToRemove[name] = true
	}

	// Filter out servers to remove
	originalCount := len(catalog.Servers)
	filtered := make([]Server, 0, len(catalog.Servers))
	for _, server := range catalog.Servers {
		if server.Snapshot == nil || !namesToRemove[server.Snapshot.Server.Name] {
			filtered = append(filtered, server)
		}
	}

	removedCount := originalCount - len(filtered)
	if removedCount == 0 {
		return fmt.Errorf("no matching servers found to remove")
	}

	catalog.Servers = filtered

	// Save the updated catalog
	dbCatalogUpdated, err := catalog.ToDb()
	if err != nil {
		return fmt.Errorf("failed to convert catalog to database format: %w", err)
	}

	if err := dao.UpsertCatalog(ctx, dbCatalogUpdated); err != nil {
		return fmt.Errorf("failed to update catalog: %w", err)
	}

	fmt.Printf("Removed %d server(s) from catalog '%s'\n", removedCount, catalogRef)
	return nil
}
