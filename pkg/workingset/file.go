package workingset

// WorkingSet represents a collection of MCP servers and their configurations
type WorkingSet struct {
	Version int               `yaml:"version" json:"version"`
	ID      string            `yaml:"id" json:"id"`
	Name    string            `yaml:"name" json:"name"`
	Servers []Server          `yaml:"servers" json:"servers"`
	Secrets map[string]Secret `yaml:"secrets,omitempty" json:"secrets,omitempty"`
}

type ServerType string

const (
	ServerTypeRegistry ServerType = "registry"
	ServerTypeImage    ServerType = "image"
)

// Server represents a server configuration in a working set
type Server struct {
	Type    ServerType             `yaml:"type" json:"type"`
	Config  map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
	Secrets string                 `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Tools   []string               `yaml:"tools,omitempty" json:"tools,omitempty"`

	// ServerTypeRegistry only
	Source string `yaml:"source,omitempty" json:"source,omitempty"`

	// ServerTypeImage only
	Image string `yaml:"image,omitempty" json:"image,omitempty"`
}

type SecretProvider string

const (
	SecretProviderCredstore SecretProvider = "docker-desktop-store"
)

// Secret represents a secret configuration in a working set
type Secret struct {
	Provider SecretProvider `yaml:"provider" json:"provider"`
}
