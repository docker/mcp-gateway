package catalognext

import (
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/validate"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

type CatalogArtifact struct {
	Title   string   `yaml:"title" json:"title" validate:"required,min=1"`
	Servers []Server `yaml:"servers" json:"servers" validate:"dive"`
}

type Catalog struct {
	Ref             string `yaml:"ref" json:"ref" validate:"required,min=1"`
	Source          string `yaml:"source,omitempty" json:"source,omitempty"`
	CatalogArtifact `yaml:",inline"`
}

type CatalogWithDigest struct {
	Catalog `yaml:",inline"`
	Digest  string `yaml:"digest" json:"digest"`
}

// Source prefixes must be of the form "<prefix>:"
const (
	SourcePrefixWorkingSet    = "profile:"
	SourcePrefixLegacyCatalog = "legacy-catalog:"
	SourcePrefixOCI           = "oci:"
)

type Server struct {
	Type  workingset.ServerType `yaml:"type" json:"type" validate:"required,oneof=registry image remote"`
	Tools []string              `yaml:"tools,omitempty" json:"tools,omitempty"`

	// ServerTypeRegistry only
	Source string `yaml:"source,omitempty" json:"source,omitempty" validate:"required_if=Type registry"`

	// ServerTypeImage only
	Image string `yaml:"image,omitempty" json:"image,omitempty" validate:"required_if=Type image"`

	// ServerTypeRemote only
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty" validate:"required_if=Type remote"`

	Snapshot *workingset.ServerSnapshot `yaml:"snapshot,omitempty" json:"snapshot,omitempty"`
}

func NewFromDb(dbCatalog *db.Catalog) CatalogWithDigest {
	servers := make([]Server, len(dbCatalog.Servers))
	for i, server := range dbCatalog.Servers {
		servers[i] = Server{
			Type:  workingset.ServerType(server.ServerType),
			Tools: server.Tools,
		}
		if server.ServerType == "registry" {
			servers[i].Source = server.Source
		}
		if server.ServerType == "image" {
			servers[i].Image = server.Image
		}
		if server.ServerType == "remote" {
			servers[i].Endpoint = server.Endpoint
		}
		if server.Snapshot != nil {
			servers[i].Snapshot = &workingset.ServerSnapshot{
				Server: server.Snapshot.Server,
			}
		}
	}

	catalog := CatalogWithDigest{
		Catalog: Catalog{
			Ref:    dbCatalog.Ref,
			Source: dbCatalog.Source,
			CatalogArtifact: CatalogArtifact{
				Title:   dbCatalog.Title,
				Servers: servers,
			},
		},
		Digest: dbCatalog.Digest,
	}

	return catalog
}

func (catalog Catalog) ToDb() (db.Catalog, error) {
	dbServers := make([]db.CatalogServer, len(catalog.Servers))
	for i, server := range catalog.Servers {
		dbServers[i] = db.CatalogServer{
			ServerType: string(server.Type),
			Tools:      server.Tools,
		}
		if server.Type == workingset.ServerTypeRegistry {
			dbServers[i].Source = server.Source
		}
		if server.Type == workingset.ServerTypeImage {
			dbServers[i].Image = server.Image
		}
		if server.Type == workingset.ServerTypeRemote {
			dbServers[i].Endpoint = server.Endpoint
		}
		if server.Snapshot != nil {
			dbServers[i].Snapshot = &db.ServerSnapshot{
				Server: server.Snapshot.Server,
			}
		}
	}

	digest, err := catalog.Digest()
	if err != nil {
		return db.Catalog{}, fmt.Errorf("failed to get catalog digest: %w", err)
	}

	return db.Catalog{
		Ref:     catalog.Ref,
		Digest:  digest,
		Title:   catalog.Title,
		Source:  catalog.Source,
		Servers: dbServers,
	}, nil
}

func (catalogArtifact *CatalogArtifact) Digest() (string, error) {
	return oci.GetArtifactDigest(MCPCatalogArtifactType, catalogArtifact)
}

func (catalog *Catalog) Validate() error {
	if err := validate.Get().Struct(catalog); err != nil {
		return err
	}
	return catalog.validateUniqueServerNames()
}

func (catalog *Catalog) validateUniqueServerNames() error {
	seen := make(map[string]bool)
	for _, server := range catalog.Servers {
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

type PullOption string

const (
	PullOptionMissing = "missing"
	PullOptionNever   = "never"
	PullOptionAlways  = "always"

	// Special value for duration-based pull options. Don't add as supported pull option below.
	PullOptionDuration = "duration"
)

func SupportedPullOptions() []string {
	return []string{string(PullOptionMissing), string(PullOptionNever), string(PullOptionAlways)}
}
