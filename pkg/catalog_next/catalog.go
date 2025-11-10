package catalognext

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/validate"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

// Any time new fields are added, make sure to include them in the Digest() method.
// This is used to ensure that the catalog is unique and can be used as a key in a database.
type Catalog struct {
	Name    string   `yaml:"name" json:"name" validate:"required,min=1"`
	Source  string   `yaml:"source,omitempty" json:"source,omitempty"`
	Servers []Server `yaml:"servers" json:"servers" validate:"dive"`
}

type CatalogWithDigest struct {
	Catalog `yaml:",inline"`
	Digest  string `yaml:"digest" json:"digest"`
}

type Server struct {
	Type  workingset.ServerType `yaml:"type" json:"type" validate:"required,oneof=registry image"`
	Tools []string              `yaml:"tools,omitempty" json:"tools,omitempty"`

	// ServerTypeRegistry only
	Source string `yaml:"source,omitempty" json:"source,omitempty" validate:"required_if=Type registry"`

	// ServerTypeImage only
	Image string `yaml:"image,omitempty" json:"image,omitempty" validate:"required_if=Type image"`

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
		if server.Snapshot != nil {
			servers[i].Snapshot = &workingset.ServerSnapshot{
				Server: server.Snapshot.Server,
			}
		}
	}

	catalog := CatalogWithDigest{
		Catalog: Catalog{
			Name:    dbCatalog.Name,
			Source:  dbCatalog.Source,
			Servers: servers,
		},
		Digest: dbCatalog.Digest,
	}

	return catalog
}

func (catalog Catalog) ToDb() db.Catalog {
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
		if server.Snapshot != nil {
			dbServers[i].Snapshot = &db.ServerSnapshot{
				Server: server.Snapshot.Server,
			}
		}
	}

	return db.Catalog{
		Digest:  catalog.Digest(),
		Name:    catalog.Name,
		Source:  catalog.Source,
		Servers: dbServers,
	}
}

func (catalog *Catalog) Digest() string {
	h := sha256.New()
	// Exclude "Source" from digest, since it's metadata and not part of the catalog's content
	h.Write([]byte(catalog.Name))
	h.Write([]byte("\n"))
	for _, server := range catalog.Servers {
		h.Write([]byte(server.Type))
		h.Write([]byte("\n"))
		h.Write([]byte(server.Source))
		h.Write([]byte("\n"))
		h.Write([]byte(server.Image))
		h.Write([]byte("\n"))
		h.Write([]byte(strings.Join(server.Tools, ",")))
		h.Write([]byte("\n"))
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

func (catalog *Catalog) Validate() error {
	return validate.Get().Struct(catalog)
}
