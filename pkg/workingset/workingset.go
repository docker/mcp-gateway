package workingset

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/docker/mcp-gateway/pkg/db"
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
	Type    ServerType             `yaml:"type" json:"type" validate:"required,oneof=registry image"`
	Config  map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
	Secrets string                 `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Tools   []string               `yaml:"tools,omitempty" json:"tools,omitempty"`

	// ServerTypeRegistry only
	Source string `yaml:"source,omitempty" json:"source,omitempty" validate:"required_if=Type registry"`

	// ServerTypeImage only
	Image string `yaml:"image,omitempty" json:"image,omitempty" validate:"required_if=Type image"`
}

type SecretProvider string

const (
	SecretProviderDockerDesktop SecretProvider = "docker-desktop-store"
)

// Secret represents a secret configuration in a working set
type Secret struct {
	Provider SecretProvider `yaml:"provider" json:"provider" validate:"required,oneof=docker-desktop-store"`
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

func createWorkingSetId(ctx context.Context, name string, dao db.DAO) (string, error) {
	// Replace all non-alphanumeric characters with a hyphen and make all uppercase lowercase
	re := regexp.MustCompile("[^a-zA-Z0-9]+")
	cleaned := re.ReplaceAllString(name, "-")
	baseName := strings.ToLower(cleaned)

	existingSets, err := dao.FindWorkingSetsByIdPrefix(ctx, baseName)
	if err != nil {
		return "", fmt.Errorf("failed to find working sets by name prefix: %w", err)
	}

	if len(existingSets) == 0 {
		return baseName, nil
	}

	takenIds := make(map[string]bool)
	for _, set := range existingSets {
		takenIds[set.ID] = true
	}

	// TODO(cody): there are better ways to do this, but this is a simple brute force for now
	// Append a number to the base name
	for i := 2; i <= 100; i++ {
		newName := fmt.Sprintf("%s-%d", baseName, i)
		if !takenIds[newName] {
			return newName, nil
		}
	}

	return "", fmt.Errorf("failed to create working set id: %w", err)
}
