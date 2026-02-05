package catalog

import "github.com/docker/mcp-gateway/pkg/policy"

type Catalog struct {
	Servers map[string]Server
}

// catalog.json

type topLevel struct {
	Name        string            `yaml:"name,omitempty" json:"name,omitempty"`
	DisplayName string            `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	Registry    map[string]Server `json:"registry"`
	// Policy describes the catalog policy decision for this catalog.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`
}

// MCP Servers

type Server struct {
	Name           string    `yaml:"name,omitempty" json:"name,omitempty" validate:"required,min=1"`
	Type           string    `yaml:"type" json:"type" validate:"required,oneof=server remote poci"`
	Image          string    `yaml:"image" json:"image"`
	Description    string    `yaml:"description,omitempty" json:"description,omitempty"`
	Title          string    `yaml:"title,omitempty" json:"title,omitempty"`
	Icon           string    `yaml:"icon,omitempty" json:"icon,omitempty"`
	ReadmeURL      string    `yaml:"readme,omitempty" json:"readme,omitempty"`
	LongLived      bool      `yaml:"longLived,omitempty" json:"longLived,omitempty"`
	Remote         Remote    `yaml:"remote" json:"remote"`
	SSEEndpoint    string    `yaml:"sseEndpoint,omitempty" json:"sseEndpoint,omitempty"` // Deprecated: Use Remote instead
	OAuth          *OAuth    `yaml:"oauth,omitempty" json:"oauth,omitempty"`
	Secrets        []Secret  `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Env            []Env     `yaml:"env,omitempty" json:"env,omitempty"`
	Command        []string  `yaml:"command,omitempty" json:"command,omitempty"`
	Volumes        []string  `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	User           string    `yaml:"user,omitempty" json:"user,omitempty"`
	DisableNetwork bool      `yaml:"disableNetwork,omitempty" json:"disableNetwork,omitempty"`
	AllowHosts     []string  `yaml:"allowHosts,omitempty" json:"allowHosts,omitempty"`
	ExtraHosts     []string  `yaml:"extraHosts,omitempty" json:"extraHosts,omitempty"`
	Tools          []Tool    `yaml:"tools,omitempty" json:"tools,omitempty" validate:"dive"`
	Config         []any     `yaml:"config,omitempty" json:"config,omitempty"`
	Prefix         string    `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	Metadata       *Metadata `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	// Policy describes the policy decision for this server.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`
}

type Metadata struct {
	Pulls       int      `yaml:"pulls,omitempty" json:"pulls,omitempty"`
	Stars       int      `yaml:"stars,omitempty" json:"stars,omitempty"`
	GithubStars int      `yaml:"githubStars,omitempty" json:"githubStars,omitempty"`
	Category    string   `yaml:"category,omitempty" json:"category,omitempty"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	License     string   `yaml:"license,omitempty" json:"license,omitempty"`
	Owner       string   `yaml:"owner,omitempty" json:"owner,omitempty"`
	// RegistryURL is the full URL to the server in the community MCP registry
	// e.g., "https://registry.modelcontextprotocol.io/v0/servers/io.github.arm%2Farm-mcp/versions/1.0.2"
	RegistryURL string `yaml:"registryUrl,omitempty" json:"registryUrl,omitempty"`
}

func (s *Server) IsOAuthServer() bool {
	return s.OAuth != nil && len(s.OAuth.Providers) > 0
}

func (s *Server) IsRemoteOAuthServer() bool {
	return s.Type == "remote" && s.IsOAuthServer()
}

type Secret struct {
	Name string `yaml:"name" json:"name"`
	Env  string `yaml:"env" json:"env"`
}

type Env struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

type Remote struct {
	URL       string            `yaml:"url,omitempty" json:"url,omitempty"`
	Transport string            `yaml:"transport_type,omitempty" json:"transport_type,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

type OAuth struct {
	Providers []OAuthProvider `yaml:"providers,omitempty" json:"providers,omitempty"`
	Scopes    []string        `yaml:"scopes,omitempty" json:"scopes,omitempty"`
}

type OAuthProvider struct {
	Provider string `yaml:"provider" json:"provider"`
	Secret   string `json:"secret,omitempty" yaml:"secret,omitempty"`
	Env      string `json:"env,omitempty" yaml:"env,omitempty"`
}

// POCI tools

type Items struct {
	Type string `yaml:"type" json:"type"`
}

type Run struct {
	Command []string          `yaml:"command,omitempty" json:"command,omitempty"`
	Volumes []string          `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

type Config struct {
	Secrets []Secret `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Env     []Env    `yaml:"env,omitempty" json:"env,omitempty"`
}

type ToolGroup struct {
	Name  string `yaml:"name" json:"name"`
	Tools []Tool `yaml:"tools" json:"tools"`
}

type Tool struct {
	Name        string `yaml:"name" json:"name" validate:"required,min=1"`
	Description string `yaml:"description" json:"description"`

	// These will only be set for oci catalogs (not legacy catalogs).
	Arguments   *[]ToolArgument  `yaml:"arguments,omitempty" json:"arguments,omitempty"`
	Annotations *ToolAnnotations `yaml:"annotations,omitempty" json:"annotations,omitempty"`

	// This is only used for POCIs.
	Container  Container  `yaml:"container,omitempty" json:"container,omitempty"`
	Parameters Parameters `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	// Policy describes the policy decision for this tool.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`
}

type Parameters struct {
	Type       string     `yaml:"type" json:"type"`
	Properties Properties `yaml:"properties" json:"properties"`
	Required   []string   `yaml:"required" json:"required"`
}

type Properties map[string]Property

type Property struct {
	Type        string `yaml:"type" json:"type"`
	Description string `yaml:"description" json:"description"`
	Items       *Items `yaml:"items,omitempty" json:"items,omitempty"`
}

type Container struct {
	Image   string   `yaml:"image" json:"image"`
	Command []string `yaml:"command" json:"command"`
	Volumes []string `yaml:"volumes" json:"volumes"`
	User    string   `yaml:"user,omitempty" json:"user,omitempty"`
}

func (p *Properties) ToMap() map[string]any {
	m := map[string]any{}

	for k, v := range *p {
		propMap := map[string]any{
			"type":        v.Type,
			"description": v.Description,
		}

		// Include items property for arrays
		if v.Type == "array" && v.Items != nil {
			propMap["items"] = map[string]any{
				"type": v.Items.Type,
			}
		}

		m[k] = propMap
	}

	return m
}

type ToolArgument struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	Items       *Items `json:"items,omitempty" yaml:"items,omitempty"`
	Description string `json:"desc" yaml:"desc"`
	Optional    bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
}

type ToolAnnotations struct {
	Title           string `json:"title,omitempty" yaml:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty" yaml:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty" yaml:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty" yaml:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty" yaml:"openWorldHint,omitempty"`
}

// Config

type ServerConfig struct {
	Name    string
	Spec    Server
	Config  map[string]any
	Secrets map[string]string
}

// IsRemote returns true if this server is a remote MCP server (not a Docker container)
func (sc *ServerConfig) IsRemote() bool {
	return sc.Spec.SSEEndpoint != "" || sc.Spec.Remote.URL != ""
}
