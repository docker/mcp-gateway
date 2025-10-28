package workingset

type Catalog struct {
	Name    string          `json:"name"`
	Servers []CatalogServer `json:"servers"`
}

type CatalogServer struct {
	Type   string         `yaml:"type" json:"type"`
	Config map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
	Tools  []string       `yaml:"tools,omitempty" json:"tools,omitempty"`

	// If type is "registry"
	Source string `yaml:"source,omitempty" json:"source,omitempty"`

	// If type is "image"
	Image string `yaml:"image,omitempty" json:"image,omitempty"`
}

func NewCatalogFromWorkingSet(workingSet WorkingSet) Catalog {
	servers := make([]CatalogServer, len(workingSet.Servers))
	for i, server := range workingSet.Servers {
		servers[i] = CatalogServer{
			Type:   string(server.Type),
			Config: server.Config,
			Tools:  server.Tools,
		}
		if server.Type == ServerTypeRegistry {
			servers[i].Source = server.Source
		}
		if server.Type == ServerTypeImage {
			servers[i].Image = server.Image
		}
	}
	return Catalog{
		Name:    workingSet.Name,
		Servers: servers,
	}
}

func (catalog Catalog) ToWorkingSet() WorkingSet {
	servers := make([]Server, len(catalog.Servers))
	for i, server := range catalog.Servers {
		servers[i] = Server{
			Type:    ServerType(server.Type),
			Config:  server.Config,
			Secrets: "default", // use default secrets
			Tools:   server.Tools,
		}
		switch server.Type {
		case string(ServerTypeRegistry):
			servers[i].Source = server.Source
		case string(ServerTypeImage):
			servers[i].Image = server.Image
		}
	}

	// Add default secrets
	secrets := make(map[string]Secret)
	secrets["default"] = Secret{
		Provider: SecretProviderDockerDesktop,
	}

	return WorkingSet{
		Version: CurrentWorkingSetVersion,
		Name:    catalog.Name,
		Servers: servers,
		Secrets: secrets,
	}
}
