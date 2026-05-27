package catalog

// ImportedServer is the schema used to parse server descriptions that
// arrive as OCI image labels (io.docker.server.metadata) — when an OCI
// catalog artifact pulls per-server snapshots from each image's label.
//
// Catalog files (loaded via --catalog or file://) continue to parse
// directly into Server, since they originate from the operator's
// configured catalog sources. Image-label content originates from
// whoever published the image and goes through this narrower type,
// which omits runtime-shaping fields (Command, Volumes, User,
// ExtraHosts, AllowHosts, DisableNetwork, Remote endpoint config,
// SSE endpoint, OAuth providers, LongLived, Policy, Secrets, and
// Env values).
//
// Secrets are intentionally excluded: the OCI label should not be able
// to dictate which secret names are looked up or which env vars receive
// injected values. Secret bindings are established only through curated
// catalogs or user configuration at server-add time.
//
// Tool definitions imported through this type also omit POCI Container
// config — POCI runtime spec is catalog-only.
type ImportedServer struct {
	Name        string         `yaml:"name,omitempty" json:"name,omitempty"`
	Type        string         `yaml:"type,omitempty" json:"type,omitempty"`
	Image       string         `yaml:"image,omitempty" json:"image,omitempty"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Title       string         `yaml:"title,omitempty" json:"title,omitempty"`
	Icon        string         `yaml:"icon,omitempty" json:"icon,omitempty"`
	ReadmeURL   string         `yaml:"readme,omitempty" json:"readme,omitempty"`
	Env         []ImportedEnv  `yaml:"env,omitempty" json:"env,omitempty"`
	Tools       []ImportedTool `yaml:"tools,omitempty" json:"tools,omitempty"`
	Config      []any          `yaml:"config,omitempty" json:"config,omitempty"`
	Prefix      string         `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	Metadata    *Metadata      `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// ImportedEnv is the env declaration schema for imported sources. Only the
// variable name is recognised; values are sourced from user configuration
// at runtime.
type ImportedEnv struct {
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
}

// ImportedTool is the tool metadata schema for imported sources. The
// Container field is intentionally absent — POCI container config (image,
// command, volumes, user) is sourced only from catalogs.
type ImportedTool struct {
	Name        string           `yaml:"name,omitempty" json:"name,omitempty"`
	Description string           `yaml:"description,omitempty" json:"description,omitempty"`
	Arguments   *[]ToolArgument  `yaml:"arguments,omitempty" json:"arguments,omitempty"`
	Annotations *ToolAnnotations `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	Parameters  Parameters       `yaml:"parameters,omitempty" json:"parameters,omitempty"`
}

// ToServer returns the Server view of this imported spec. All
// runtime-shaping fields on the result are zero-valued.
func (i ImportedServer) ToServer() Server {
	s := Server{
		Name:        i.Name,
		Type:        i.Type,
		Image:       i.Image,
		Description: i.Description,
		Title:       i.Title,
		Icon:        i.Icon,
		ReadmeURL:   i.ReadmeURL,
		Config:      i.Config,
		Prefix:      i.Prefix,
		Metadata:    i.Metadata,
	}
	if len(i.Env) > 0 {
		s.Env = make([]Env, len(i.Env))
		for k, e := range i.Env {
			s.Env[k] = Env{Name: e.Name}
		}
	}
	if len(i.Tools) > 0 {
		s.Tools = make([]Tool, len(i.Tools))
		for k, t := range i.Tools {
			s.Tools[k] = Tool{
				Name:        t.Name,
				Description: t.Description,
				Arguments:   t.Arguments,
				Annotations: t.Annotations,
				Parameters:  t.Parameters,
			}
		}
	}
	return s
}
