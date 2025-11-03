package workingset

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
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
	Tools   []string       `yaml:"tools,omitempty" json:"tools,omitempty"`

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
	return validate.Get().Struct(workingSet)
}

func createWorkingSetID(ctx context.Context, name string, dao db.DAO) (string, error) {
	// Replace all non-alphanumeric characters with a hyphen and make all uppercase lowercase
	re := regexp.MustCompile("[^a-zA-Z0-9]+")
	cleaned := re.ReplaceAllString(name, "-")
	baseName := strings.ToLower(cleaned)

	existingSets, err := dao.FindWorkingSetsByIDPrefix(ctx, baseName)
	if err != nil {
		return "", fmt.Errorf("failed to find working sets by name prefix: %w", err)
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

	return "", fmt.Errorf("failed to create working set id")
}

func resolveServerFromString(ctx context.Context, registryClient registryapi.Client, ociService oci.Service, value string) (Server, error) {
	if v, ok := strings.CutPrefix(value, "docker://"); ok {
		return resolveImage(ctx, ociService, v)
	} else if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") { // Assume registry entry if it's a URL
		return resolveRegistry(ctx, registryClient, value)
	}
	return Server{}, fmt.Errorf("invalid server value: %s", value)
}

func resolveImage(ctx context.Context, ociService oci.Service, value string) (Server, error) {
	ref, err := name.ParseReference(value)
	if err != nil {
		return Server{}, fmt.Errorf("failed to parse reference: %w", err)
	}
	digest, err := ociService.GetImageDigest(ctx, ref)
	if err != nil {
		return Server{}, fmt.Errorf("failed to get image digest: %w", err)
	}
	var refWithDigest string
	if hasDigest(ref) {
		refWithDigest = ref.String()
	} else {
		refWithDigest = fmt.Sprintf("%s@%s", ref, digest)
	}

	serverSnapshot, err := getCatalogServerFromImage(ctx, ociService, ref)
	if err != nil {
		return Server{}, fmt.Errorf("failed to validate self-contained image: %w", err)
	}

	return Server{
		Type:    ServerTypeImage,
		Image:   refWithDigest,
		Secrets: "default",
		Snapshot: &ServerSnapshot{
			Server: serverSnapshot,
		},
	}, nil
}

func resolveRegistry(ctx context.Context, registryClient registryapi.Client, value string) (Server, error) {
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

	var server *v0.ServerResponse
	for _, version := range versions.Servers {
		if version.Server.Version == url.Version {
			server = &version
			break
		}
	}
	if server == nil {
		return Server{}, fmt.Errorf("server version not found")
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
		return Server{}, fmt.Errorf("oci package not found for server %s", url.String())
	}

	return Server{
		Type:    ServerTypeRegistry,
		Source:  url.String(),
		Secrets: "default",
		// TODO(cody): add snapshot
	}, nil
}

func ResolveSnapshot(ctx context.Context, ociService oci.Service, server Server) (*ServerSnapshot, error) {
	switch server.Type {
	case ServerTypeImage:
		return resolveImageSnapshot(ctx, ociService, server)
	case ServerTypeRegistry:
		// TODO(cody): add snapshot
		return nil, nil //nolint:nilnil
	}
	return nil, fmt.Errorf("unsupported server type: %s", server.Type)
}

func resolveImageSnapshot(ctx context.Context, ociService oci.Service, server Server) (*ServerSnapshot, error) {
	ref, err := name.ParseReference(server.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	serverSnapshot, err := getCatalogServerFromImage(ctx, ociService, ref)
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

func getCatalogServerFromImage(ctx context.Context, ociService oci.Service, ref name.Reference) (catalog.Server, error) {
	labels, err := ociService.GetImageLabels(ctx, ref)
	if err != nil {
		return catalog.Server{}, fmt.Errorf("failed to get image labels: %w", err)
	}
	metadataLabel := labels["io.docker.server.metadata"]
	if metadataLabel == "" {
		return catalog.Server{}, fmt.Errorf("image %s is not a self-describing image", fullName(ref))
	}

	// Basic parsing validation
	var server catalog.Server
	if err := yaml.Unmarshal([]byte(metadataLabel), &server); err != nil {
		return catalog.Server{}, fmt.Errorf("failed to parse metadata label for %s: %w", fullName(ref), err)
	}

	return server, nil
}
