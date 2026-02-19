package workingset

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/policy"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/pkg/sliceutil"
	"github.com/docker/mcp-gateway/pkg/validate"
)

const CurrentWorkingSetVersion = 1

// WorkingSet represents a collection of MCP servers and their configurations
type WorkingSet struct {
	Version int               `yaml:"version" json:"version" validate:"required,min=1,max=1"`
	ID      string            `yaml:"id" json:"id" validate:"required"`
	Name    string            `yaml:"name" json:"name" validate:"required,min=1"`
	Servers []Server          `yaml:"servers" json:"servers" validate:"dive"`
	Secrets map[string]Secret `yaml:"secrets,omitempty" json:"secrets,omitempty" validate:"dive"`
	// Policy describes the policy decision for this working set.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`
}

type ServerType string

const (
	ServerTypeRegistry ServerType = "registry"
	ServerTypeImage    ServerType = "image"
	ServerTypeRemote   ServerType = "remote"
)

// Server represents a server configuration in a working set
type Server struct {
	Type    ServerType     `yaml:"type" json:"type" validate:"required,oneof=registry image remote"`
	Config  map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
	Secrets string         `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Tools   ToolList       `yaml:"tools,omitempty" json:"tools"` // See IsZero() below
	// Policy describes the policy decision for this server.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`

	// ServerTypeRegistry only
	Source string `yaml:"source,omitempty" json:"source,omitempty" validate:"required_if=Type registry"`

	// ServerTypeImage only
	Image string `yaml:"image,omitempty" json:"image,omitempty" validate:"required_if=Type image"`

	// ServerTypeRemote only
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty" validate:"required_if=Type remote"`

	// Optional snapshot of the server schema
	Snapshot *ServerSnapshot `yaml:"snapshot,omitempty" json:"snapshot,omitempty"`
}

type SecretProvider string

const (
	SecretProviderDockerDesktop SecretProvider = "docker-desktop-store"
)

// Secret represents a secret configuration in a working set
type Secret struct {
	Provider SecretProvider `yaml:"provider" json:"provider" validate:"required,oneof=docker-desktop-store"`
}

type ServerSnapshot struct {
	Server catalog.Server `yaml:"server" json:"server"`
}

type ToolList []string

// Needed for proper YAML encoding with omitempty. YAML defaults IsZero to true when a slice is empty, but we only want it on nil.
// This IsZero() + omitempty matches json behavior without omitempty.
func (tools ToolList) IsZero() bool {
	return tools == nil
}

func NewFromDb(dbSet *db.WorkingSet) WorkingSet {
	servers := make([]Server, len(dbSet.Servers))
	for i, server := range dbSet.Servers {
		servers[i] = Server{
			Type:    ServerType(server.Type),
			Config:  server.Config,
			Secrets: server.Secrets,
			Tools:   server.Tools,
		}
		if server.Type == "registry" {
			servers[i].Source = server.Source
		}
		if server.Type == "image" {
			servers[i].Image = server.Image
		}
		if server.Type == "remote" {
			servers[i].Endpoint = server.Endpoint
		}

		if server.Snapshot != nil {
			servers[i].Snapshot = &ServerSnapshot{
				Server: server.Snapshot.Server,
			}
		}
	}

	secrets := make(map[string]Secret)
	for name, secret := range dbSet.Secrets {
		secrets[name] = Secret{
			Provider: SecretProvider(secret.Provider),
		}
	}

	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      dbSet.ID,
		Name:    dbSet.Name,
		Servers: servers,
		Secrets: secrets,
	}

	return workingSet
}

func (workingSet WorkingSet) ToDb() db.WorkingSet {
	dbServers := make(db.ServerList, len(workingSet.Servers))
	for i, server := range workingSet.Servers {
		dbServers[i] = db.Server{
			Type:    string(server.Type),
			Config:  server.Config,
			Secrets: server.Secrets,
			Tools:   server.Tools,
		}
		if server.Type == ServerTypeRegistry {
			dbServers[i].Source = server.Source
		}
		if server.Type == ServerTypeImage {
			dbServers[i].Image = server.Image
		}
		if server.Type == ServerTypeRemote {
			dbServers[i].Endpoint = server.Endpoint
		}
		if server.Snapshot != nil {
			dbServers[i].Snapshot = &db.ServerSnapshot{
				Server: server.Snapshot.Server,
			}
		}
	}

	dbSecrets := make(db.SecretMap, len(workingSet.Secrets))
	for name, secret := range workingSet.Secrets {
		dbSecrets[name] = db.Secret{
			Provider: string(secret.Provider),
		}
	}

	dbSet := db.WorkingSet{
		ID:      workingSet.ID,
		Name:    workingSet.Name,
		Servers: dbServers,
		Secrets: dbSecrets,
	}

	return dbSet
}

func (workingSet *WorkingSet) Validate() error {
	if err := validate.Get().Struct(workingSet); err != nil {
		return err
	}
	if err := workingSet.validateUniqueServerNames(); err != nil {
		return err
	}
	return workingSet.validateServerSnapshots()
}

func (workingSet *WorkingSet) validateUniqueServerNames() error {
	seen := make(map[string]bool)
	for _, server := range workingSet.Servers {
		// TODO: Update when Snapshot is required
		if server.Snapshot == nil {
			continue
		}
		name := server.Snapshot.Server.Name
		if seen[name] {
			return fmt.Errorf("duplicate server name %s", name)
		}
		seen[name] = true
	}
	return nil
}

func (workingSet *WorkingSet) validateServerSnapshots() error {
	for _, server := range workingSet.Servers {
		if err := server.Snapshot.ValidateInnerConfig(); err != nil {
			return err
		}
	}
	return nil
}

func (serverSnapshot *ServerSnapshot) ValidateInnerConfig() error {
	if serverSnapshot == nil {
		return nil
	}

	config := serverSnapshot.Server.Config
	if config == nil {
		return nil
	}

	for i, configItem := range config {
		configMap, ok := configItem.(map[string]any)
		if !ok {
			return fmt.Errorf("config[%d] is not a map", i)
		}

		_, ok = configMap["name"].(string)
		if !ok {
			return fmt.Errorf("config[%d] has no name field", i)
		}

		_, ok = configMap["description"].(string)
		if !ok {
			return fmt.Errorf("config[%d] has no description field", i)
		}

		t, ok := configMap["type"].(string)
		if !ok {
			return fmt.Errorf("config[%d] has no type field", i)
		}
		if t != "object" {
			return fmt.Errorf("config[%d].type must be 'object', got '%s'", i, t)
		}

		properties, ok := configMap["properties"].(map[string]any)
		if !ok {
			return fmt.Errorf("config[%d].properties is not a map", i)
		}

		if err := recursivePropertiesValidate(properties, fmt.Sprintf("config[%d].properties", i)); err != nil {
			return err
		}
	}

	return nil
}

func recursivePropertiesValidate(properties map[string]any, path string) error {
	for key, property := range properties {
		propertyPath := fmt.Sprintf("%s.%s", path, key)

		propertyMap, ok := property.(map[string]any)
		if !ok {
			return fmt.Errorf("%s is not a map", propertyPath)
		}

		t, ok := propertyMap["type"].(string)
		if !ok {
			return fmt.Errorf("%s has no type field", propertyPath)
		}

		switch t {
		case "string", "integer", "number", "boolean":
			continue
		case "object":
			innerProperties, ok := propertyMap["properties"].(map[string]any)
			if !ok {
				return fmt.Errorf("%s is type 'object' but has no properties field", propertyPath)
			}
			if err := recursivePropertiesValidate(innerProperties, propertyPath); err != nil {
				return err
			}
		case "array":
			items, ok := propertyMap["items"].(map[string]any)
			if !ok {
				return fmt.Errorf("%s is type 'array' but has no items field", propertyPath)
			}
			itemType, ok := items["type"].(string)
			if !ok {
				return fmt.Errorf("%s.items has no type field", propertyPath)
			}
			if itemType != "string" {
				return fmt.Errorf("%s.items type must be string", propertyPath)
			}
		default:
			return fmt.Errorf("%s.type %s is not supported", propertyPath, t)
		}
	}
	return nil
}

func (workingSet *WorkingSet) FindServer(serverName string) *Server {
	for i := range len(workingSet.Servers) {
		if workingSet.Servers[i].Snapshot == nil {
			// TODO(cody): Can happen with registry (for now)
			continue
		}
		if workingSet.Servers[i].Snapshot.Server.Name == serverName {
			return &workingSet.Servers[i]
		}
	}
	return nil
}

func (workingSet *WorkingSet) EnsureSnapshotsResolved(ctx context.Context, ociService oci.Service) error {
	// Ensure all snapshots are resolved
	for i := range len(workingSet.Servers) {
		if workingSet.Servers[i].Snapshot != nil {
			continue
		}
		log.Log(fmt.Sprintf("Server %s has no snapshot, lazy loading the snapshot...\n", workingSet.Servers[i].BasicName()))
		snapshot, err := ResolveSnapshot(ctx, ociService, workingSet.Servers[i])
		if err != nil {
			return fmt.Errorf("failed to resolve snapshot for server[%d]: %w", i, err)
		}
		// TODO(cody): Can be nil with registry (for now)
		if snapshot != nil {
			workingSet.Servers[i].Snapshot = snapshot
		}
	}

	return nil
}

func (s *Server) BasicName() string {
	switch s.Type {
	case ServerTypeImage:
		return s.Image
	case ServerTypeRegistry:
		return s.Source
	}
	return "unknown"
}

func createWorkingSetID(ctx context.Context, name string, dao db.DAO) (string, error) {
	// Replace all non-alphanumeric characters with an underscore and make all uppercase lowercase
	re := regexp.MustCompile("[^a-zA-Z0-9]+")
	cleaned := re.ReplaceAllString(name, "_")
	baseName := strings.ToLower(cleaned)

	existingSets, err := dao.FindWorkingSetsByIDPrefix(ctx, baseName)
	if err != nil {
		return "", fmt.Errorf("failed to find profiles by name prefix: %w", err)
	}

	if len(existingSets) == 0 {
		return baseName, nil
	}

	takenIDs := make(map[string]bool)
	for _, set := range existingSets {
		takenIDs[set.ID] = true
	}

	// TODO(cody): there are better ways to do this, but this is a simple brute force for now
	// Append a number to the base name
	for i := 2; i <= 100; i++ {
		newName := fmt.Sprintf("%s_%d", baseName, i)
		if !takenIDs[newName] {
			return newName, nil
		}
	}

	return "", fmt.Errorf("failed to create profile id")
}

func ResolveServersFromString(ctx context.Context, registryClient registryapi.Client, ociService oci.Service, dao db.DAO, value string) ([]Server, error) {
	if v, ok := strings.CutPrefix(value, "docker://"); ok {
		fullRef, err := ResolveImageRef(ctx, ociService, v)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve image ref: %w", err)
		}
		serverSnapshot, err := ResolveImageSnapshot(ctx, ociService, fullRef)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve image snapshot: %w", err)
		}
		return []Server{{
			Type:     ServerTypeImage,
			Image:    fullRef,
			Secrets:  "default",
			Snapshot: serverSnapshot,
		}}, nil
	} else if v, ok := strings.CutPrefix(value, "catalog://"); ok {
		return ResolveCatalogServers(ctx, dao, v)
	} else if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") { // Assume registry entry if it's a URL
		server, err := ResolveRegistry(ctx, registryClient, value)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve registry: %w", err)
		}
		return []Server{server}, nil
	} else if v, ok := strings.CutPrefix(value, "file://"); ok {
		return ResolveFile(ctx, v)
	}
	return nil, fmt.Errorf("invalid server value: %s", value)
}

// isV0ServerJSON checks if the JSON data represents a v0.ServerJSON/v0.ServerResponse
// rather than a catalog.Server by looking for discriminating fields.
func isV0ServerJSON(buf []byte) bool {
	var discriminator struct {
		Schema   string `json:"$schema"`  // Present in v0.ServerJSON
		Type     string `json:"type"`     // Present in catalog.Server (required)
		Packages []any  `json:"packages"` // Present in v0.ServerJSON
		Remotes  []any  `json:"remotes"`  // Present in v0.ServerJSON
	}
	if err := json.Unmarshal(buf, &discriminator); err != nil {
		return false
	}

	// If it has the catalog.Server-specific "type" field, it's NOT a v0.ServerJSON
	if discriminator.Type != "" {
		return false
	}

	// If it has v0.ServerJSON-specific fields, it IS a v0.ServerJSON
	if discriminator.Schema != "" || len(discriminator.Packages) > 0 || len(discriminator.Remotes) > 0 {
		return true
	}

	return false
}

func ResolveFile(ctx context.Context, value string) ([]Server, error) {
	buf, err := os.ReadFile(value)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// First, see if it's a full legacy catalog file.
	// Fallback to a single server if it's not.
	var probe struct {
		Registry map[string]catalog.Server `yaml:"registry,omitempty" json:"registry,omitempty"`
	}

	var servers []catalog.Server
	switch filepath.Ext(strings.ToLower(value)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(buf, &probe); err != nil {
			return nil, fmt.Errorf("failed to unmarshal server: %w", err)
		}
		if probe.Registry == nil {
			// Fallback to parsing single server
			var server catalog.Server
			if err := yaml.Unmarshal(buf, &server); err != nil {
				return nil, fmt.Errorf("failed to unmarshal server: %w", err)
			}
			servers = []catalog.Server{server}
		}
	case ".json":
		if err := json.Unmarshal(buf, &probe); err != nil {
			return nil, fmt.Errorf("failed to unmarshal server: %w", err)
		}
		if probe.Registry == nil {
			// Use discriminating fields to determine type
			if isV0ServerJSON(buf) {
				// Try to parse as v0.ServerResponse first
				var serverResp v0.ServerResponse
				if err := json.Unmarshal(buf, &serverResp); err == nil && serverResp.Server.Name != "" {
					// Successfully parsed as v0.ServerResponse
					catalogServer, err := ConvertRegistryServerToCatalog(ctx, &serverResp, catalog.DefaultPyPIVersionResolver())
					if err != nil {
						return nil, fmt.Errorf("failed to convert v0.ServerResponse to catalog.Server: %w", err)
					}
					servers = []catalog.Server{catalogServer}
				} else {
					// Try to parse as v0.ServerJSON and wrap it
					var serverJSON v0.ServerJSON
					if err := json.Unmarshal(buf, &serverJSON); err == nil && serverJSON.Name != "" {
						// Successfully parsed as v0.ServerJSON, wrap it in ServerResponse
						serverResp := &v0.ServerResponse{
							Server: serverJSON,
						}
						catalogServer, err := ConvertRegistryServerToCatalog(ctx, serverResp, catalog.DefaultPyPIVersionResolver())
						if err != nil {
							return nil, fmt.Errorf("failed to convert v0.ServerJSON to catalog.Server: %w", err)
						}
						servers = []catalog.Server{catalogServer}
					} else {
						return nil, fmt.Errorf("failed to parse as v0.ServerJSON despite discriminating fields indicating v0 format")
					}
				}
			} else {
				// Parse as catalog.Server directly
				var server catalog.Server
				if err := json.Unmarshal(buf, &server); err != nil {
					return nil, fmt.Errorf("failed to unmarshal server as catalog.Server: %w", err)
				}
				servers = []catalog.Server{server}
			}
		}
	default:
		return nil, fmt.Errorf("unsupported file extension: %s, must be .yaml or .json", value)
	}

	if probe.Registry != nil {
		for name, server := range probe.Registry {
			server.Name = name
			servers = append(servers, server)
		}
	}

	serversResolved := make([]Server, len(servers))
	for i, server := range servers {
		if (server.Type == "server" || server.Type == "poci") && server.Image != "" {
			serversResolved[i] = Server{
				Type:     ServerTypeImage,
				Image:    server.Image,
				Secrets:  "default",
				Snapshot: &ServerSnapshot{Server: server},
			}
		} else if server.Type == "remote" {
			serversResolved[i] = Server{
				Type:     ServerTypeRemote,
				Endpoint: server.Remote.URL,
				Secrets:  "default",
				Snapshot: &ServerSnapshot{Server: server},
			}
		} else {
			return nil, fmt.Errorf("unsupported server type: %s", server.Type)
		}
	}

	return serversResolved, nil
}

func ResolveCatalogServers(ctx context.Context, dao db.DAO, value string) ([]Server, error) {
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid catalog URL: catalog://%s", value)
	}
	catalogRef := strings.Join(parts[:len(parts)-1], "/")
	serverList := parts[len(parts)-1]

	serverNames := strings.Split(serverList, "+")

	if len(serverNames) == 0 || len(serverList) == 0 {
		return nil, fmt.Errorf("no servers specified in catalog URL: catalog://%s", value)
	}

	ref, err := name.ParseReference(catalogRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse catalog reference %s: %w", catalogRef, err)
	}
	catalogRef = oci.FullNameWithoutDigest(ref)

	dbCatalog, err := dao.GetCatalog(ctx, catalogRef)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("catalog %s not found", catalogRef)
		}
		return nil, fmt.Errorf("failed to get catalog: %w", err)
	}

	filteredServers := make([]db.CatalogServer, 0, len(dbCatalog.Servers))
	foundPatterns := make(map[string]bool)
	foundServers := make(map[string]bool) // avoid duplicates
	for _, name := range serverNames {
		for _, server := range dbCatalog.Servers {
			// Glob support for selecting servers by name
			matched, err := path.Match(name, server.Snapshot.Server.Name)
			if err != nil {
				return nil, fmt.Errorf("bad pattern for catalog server '%s': %w", name, err)
			}
			if matched {
				if !foundServers[server.Snapshot.Server.Name] {
					// Only add to avoid duplicates
					filteredServers = append(filteredServers, server)
					foundServers[server.Snapshot.Server.Name] = true
				}
				foundPatterns[name] = true
			}
		}
	}

	uniqueServerNames := make(map[string]bool)
	for _, serverName := range serverNames {
		uniqueServerNames[serverName] = true
	}

	if len(foundPatterns) != len(uniqueServerNames) {
		f := make([]string, 0, len(foundPatterns))
		for pattern := range foundPatterns {
			f = append(f, pattern)
		}
		missingPatterns := sliceutil.Difference(serverNames, f)
		return nil, fmt.Errorf("servers matching the following patterns were not found in catalog: %v", missingPatterns)
	}

	return mapCatalogServersToWorkingSetServers(filteredServers, "default"), nil
}

func ResolveImageRef(ctx context.Context, ociService oci.Service, value string) (string, error) {
	ref, err := name.ParseReference(value)
	if err != nil {
		return "", fmt.Errorf("failed to parse reference: %w", err)
	}
	isRemote := false
	img, err := ociService.GetLocalImage(ctx, ref)
	if oci.IsNoSuchImageError(err) {
		img, err = ociService.GetRemoteImage(ctx, ref)
		isRemote = true
	}
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}
	var fullRef string
	if !isRemote || oci.HasDigest(ref) {
		// Local images shouldn't be referenced by a digest
		fullRef = ref.String()
	} else {
		// Remotes should be pinned to a digest
		digest, err := ociService.GetImageDigest(img)
		if err != nil {
			return "", fmt.Errorf("failed to get image digest: %w", err)
		}
		fullRef = fmt.Sprintf("%s@%s", ref.String(), digest)
	}

	return fullRef, nil
}

func ConvertRegistryServerToCatalog(ctx context.Context, serverResp *v0.ServerResponse, pypiResolver catalog.PyPIVersionResolver) (catalog.Server, error) {
	result, err := catalog.TransformToDocker(ctx, serverResp.Server, catalog.WithPyPIResolver(pypiResolver))
	if err != nil {
		return catalog.Server{}, err
	}
	return *result, nil
}

func ResolveRegistry(ctx context.Context, registryClient registryapi.Client, value string) (Server, error) {
	url, err := registryapi.ParseServerURL(value)
	if err != nil {
		return Server{}, fmt.Errorf("failed to parse server URL %s: %w", value, err)
	}

	versions, err := registryClient.GetServerVersions(ctx, url)
	if err != nil {
		return Server{}, fmt.Errorf("failed to get server versions from URL %s: %w", url.VersionsListURL(), err)
	}

	if len(versions.Servers) == 0 {
		return Server{}, fmt.Errorf("no server versions found for URL %s", url.VersionsListURL())
	}

	if url.IsLatestVersion() {
		latestVersion, err := resolveLatestVersion(versions)
		if err != nil {
			return Server{}, fmt.Errorf("failed to resolve latest version for server %s: %w", url.VersionsListURL(), err)
		}
		url = url.WithVersion(latestVersion)
	}

	var serverResp *v0.ServerResponse
	for _, version := range versions.Servers {
		if version.Server.Version == url.Version {
			serverResp = &version
			break
		}
	}
	if serverResp == nil {
		return Server{}, fmt.Errorf("server version not found")
	}

	// Check for OCI packages and convert to catalog format
	catalogServer, err := ConvertRegistryServerToCatalog(ctx, serverResp, catalog.DefaultPyPIVersionResolver())
	if err != nil {
		return Server{}, fmt.Errorf("failed to convert registry server: %w", err)
	}

	return Server{
		Type:     ServerTypeRegistry,
		Source:   url.String(),
		Secrets:  "default",
		Snapshot: &ServerSnapshot{Server: catalogServer},
	}, nil
}

func ResolveSnapshot(ctx context.Context, ociService oci.Service, server Server) (*ServerSnapshot, error) {
	switch server.Type {
	case ServerTypeImage:
		return ResolveImageSnapshot(ctx, ociService, server.Image)
	case ServerTypeRegistry:
		// Snapshots for registry servers are resolved during ResolveRegistry
		return nil, nil //nolint:nilnil
	case ServerTypeRemote:
		// TODO(bobby): add snapshot when you can add remotes directly from URL
		return nil, nil //nolint:nilnil
	}
	return nil, fmt.Errorf("unsupported server type: %s", server.Type)
}

func ResolveImageSnapshot(ctx context.Context, ociService oci.Service, image string) (*ServerSnapshot, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	var img v1.Image
	// Anything with a digest should be a remote image
	if oci.HasDigest(ref) {
		img, err = ociService.GetRemoteImage(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to get remote image: %w", err)
		}
	} else {
		img, err = ociService.GetLocalImage(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to get local image: %w", err)
		}
	}

	serverSnapshot, err := getCatalogServerFromImage(ociService, img, image)
	if err != nil {
		return nil, fmt.Errorf("failed to get catalog server from image: %w", err)
	}
	return &ServerSnapshot{
		Server: serverSnapshot,
	}, nil
}

// Pins the "latest" to a specific version
func resolveLatestVersion(versions v0.ServerListResponse) (string, error) {
	for _, version := range versions.Servers {
		if version.Meta.Official.IsLatest {
			return version.Server.Version, nil
		}
	}
	return "", fmt.Errorf("no latest version found")
}

func getCatalogServerFromImage(ociService oci.Service, img v1.Image, name string) (catalog.Server, error) {
	labels, err := ociService.GetImageLabels(img)
	if err != nil {
		return catalog.Server{}, fmt.Errorf("failed to get image labels: %w", err)
	}
	metadataLabel := labels["io.docker.server.metadata"]
	if metadataLabel == "" {
		return catalog.Server{}, fmt.Errorf("image %s is not a self-describing image", name)
	}

	// Basic parsing validation
	var server catalog.Server
	if err := yaml.Unmarshal([]byte(metadataLabel), &server); err != nil {
		return catalog.Server{}, fmt.Errorf("failed to parse metadata label for %s: %w", name, err)
	}

	server.Type = "server"
	server.Image = name

	return server, nil
}

func mapCatalogServersToWorkingSetServers(dbServers []db.CatalogServer, secrets string) []Server {
	servers := make([]Server, len(dbServers))
	for i, server := range dbServers {
		servers[i] = Server{
			Type:     ServerType(server.ServerType),
			Tools:    ToolList(server.Tools),
			Config:   map[string]any{},
			Source:   server.Source,
			Image:    server.Image,
			Endpoint: server.Endpoint,
			Snapshot: &ServerSnapshot{
				Server: server.Snapshot.Server,
			},
			Secrets: secrets,
		}
	}
	return servers
}
