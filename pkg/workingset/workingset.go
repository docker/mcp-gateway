package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oci"
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
}

type ServerType string

const (
	ServerTypeRegistry ServerType = "registry"
	ServerTypeImage    ServerType = "image"
)

// Server represents a server configuration in a working set
type Server struct {
	Type    ServerType     `yaml:"type" json:"type" validate:"required,oneof=registry image"`
	Config  map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
	Secrets string         `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Tools   []string       `yaml:"tools" json:"tools"`

	// ServerTypeRegistry only
	Source string `yaml:"source,omitempty" json:"source,omitempty" validate:"required_if=Type registry"`

	// ServerTypeImage only
	Image string `yaml:"image,omitempty" json:"image,omitempty" validate:"required_if=Type image"`

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
	err := validate.Get().Struct(workingSet)
	if err != nil {
		return err
	}
	return workingSet.validateUniqueServerNames()
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
	// Replace all non-alphanumeric characters with a hyphen and make all uppercase lowercase
	re := regexp.MustCompile("[^a-zA-Z0-9]+")
	cleaned := re.ReplaceAllString(name, "-")
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
		newName := fmt.Sprintf("%s-%d", baseName, i)
		if !takenIDs[newName] {
			return newName, nil
		}
	}

	return "", fmt.Errorf("failed to create profile id")
}

func resolveServersFromString(ctx context.Context, registryClient registryapi.Client, ociService oci.Service, dao db.DAO, value string) ([]Server, error) {
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
		url, err := ResolveRegistry(ctx, registryClient, value)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve registry: %w", err)
		}
		return []Server{{
			Type:    ServerTypeRegistry,
			Source:  url,
			Secrets: "default",
			// TODO(cody): add snapshot
		}}, nil
	}
	return nil, fmt.Errorf("invalid server value: %s", value)
}

func ResolveCatalogServers(ctx context.Context, dao db.DAO, value string) ([]Server, error) {
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid catalog URL: catalog://%s", value)
	}
	catalogRef := strings.Join(parts[:len(parts)-1], "/")
	serverList := parts[len(parts)-1]

	serverNames := strings.Split(serverList, "+")

	if len(serverNames) == 0 {
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
	for _, server := range dbCatalog.Servers {
		if slices.Contains(serverNames, server.Snapshot.Server.Name) {
			filteredServers = append(filteredServers, server)
		}
	}
	if len(filteredServers) != len(serverNames) {
		missingServers := sliceutil.Difference(serverNames, sliceutil.Map(filteredServers, func(server db.CatalogServer) string { return server.Snapshot.Server.Name }))
		return nil, fmt.Errorf("servers were not found in catalog: %v", missingServers)
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

func ResolveRegistry(ctx context.Context, registryClient registryapi.Client, value string) (string, error) {
	url, err := registryapi.ParseServerURL(value)
	if err != nil {
		return "", fmt.Errorf("failed to parse server URL %s: %w", value, err)
	}

	versions, err := registryClient.GetServerVersions(ctx, url)
	if err != nil {
		return "", fmt.Errorf("failed to get server versions from URL %s: %w", url.VersionsListURL(), err)
	}

	if len(versions.Servers) == 0 {
		return "", fmt.Errorf("no server versions found for URL %s", url.VersionsListURL())
	}

	if url.IsLatestVersion() {
		latestVersion, err := resolveLatestVersion(versions)
		if err != nil {
			return "", fmt.Errorf("failed to resolve latest version for server %s: %w", url.VersionsListURL(), err)
		}
		url = url.WithVersion(latestVersion)
	}

	var server *v0.ServerResponse
	for _, version := range versions.Servers {
		if version.Server.Version == url.Version {
			server = &version
			break
		}
	}
	if server == nil {
		return "", fmt.Errorf("server version not found")
	}

	// check oci package exists
	foundOCIPackage := false
	for _, pkg := range server.Server.Packages {
		if pkg.RegistryType == "oci" {
			foundOCIPackage = true
			break
		}
	}
	if !foundOCIPackage {
		return "", fmt.Errorf("oci package not found for server %s", url.String())
	}

	return url.String(), nil
}

func ResolveSnapshot(ctx context.Context, ociService oci.Service, server Server) (*ServerSnapshot, error) {
	switch server.Type {
	case ServerTypeImage:
		return ResolveImageSnapshot(ctx, ociService, server.Image)
	case ServerTypeRegistry:
		// TODO(cody): add snapshot
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
			Type:   ServerType(server.ServerType),
			Tools:  server.Tools,
			Config: map[string]any{},
			Source: server.Source,
			Image:  server.Image,
			Snapshot: &ServerSnapshot{
				Server: server.Snapshot.Server,
			},
			Secrets: secrets,
		}
	}
	return servers
}
