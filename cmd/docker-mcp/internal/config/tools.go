package config

import (
	"gopkg.in/yaml.v3"
)

type ToolsConfig struct {
	ServerTools map[string][]string `yaml:",inline"`
}

func ParseToolsConfig(toolsYaml []byte) (ToolsConfig, error) {
	var serverTools ToolsConfig
	if err := yaml.Unmarshal(toolsYaml, &serverTools); err != nil {
		return ToolsConfig{}, err
	}

	return serverTools, nil
}
