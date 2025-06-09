package config

import (
	"sort"

	"gopkg.in/yaml.v3"
)

type Registry struct {
	Servers map[string]Tile `yaml:"registry"`
}

type Tile struct {
	Ref    string         `yaml:"ref"`
	Config map[string]any `yaml:"config,omitempty"`
}

func ParseRegistryConfig(registryYaml []byte) (Registry, error) {
	var registry Registry
	if err := yaml.Unmarshal(registryYaml, &registry); err != nil {
		return Registry{}, err
	}

	return registry, nil
}

func (r *Registry) ServerNames() []string {
	var names []string

	for name := range r.Servers {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}
