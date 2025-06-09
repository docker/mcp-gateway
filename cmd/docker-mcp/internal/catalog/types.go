package catalog

type Catalog struct {
	Servers map[string]Server
}

// catalog.json

type topLevel struct {
	Registry map[string]Server `json:"registry"`
}

// MCP Servers

type Server struct {
	Image          string   `yaml:"image" json:"image"`
	Ref            string   `yaml:"ref" json:"ref"`
	SSEEndpoint    string   `yaml:"sseEndpoint,omitempty" json:"sseEndpoint,omitempty"`
	Secrets        []Secret `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Env            []Env    `yaml:"env,omitempty" json:"env,omitempty"`
	Command        []string `yaml:"command,omitempty" json:"command,omitempty"`
	Volumes        []string `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	DisableNetwork bool     `yaml:"disableNetwork,omitempty" json:"disableNetwork,omitempty"`
	Tools          []Tool   `yaml:"tools,omitempty" json:"tools,omitempty"`
}

type Secret struct {
	Name string `yaml:"name" json:"name"`
	Env  string `yaml:"env" json:"env"`
}

type Env struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
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
	Name        string     `yaml:"name" json:"name"`
	Description string     `yaml:"description" json:"description"`
	Container   Container  `yaml:"container" json:"container"`
	Parameters  Parameters `yaml:"parameters" json:"parameters"`
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
