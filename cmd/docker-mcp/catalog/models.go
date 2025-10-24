package catalog

type Registry struct {
	Registry map[string]Tile `yaml:"registry"`
}

type Tile struct {
	Description string        `yaml:"description"`
	ReadmeURL   string        `yaml:"readme"`
	ToolsURL    string        `yaml:"toolsUrl"`
	Dynamic     *DynamicFlags `yaml:"dynamic,omitempty"`
}

type DynamicFlags struct {
	Tools bool `yaml:"tools"`
}
