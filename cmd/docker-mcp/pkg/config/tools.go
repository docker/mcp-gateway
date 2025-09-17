package config

import (
	"gopkg.in/yaml.v3"
)

type ToolsConfig struct {
	ServerTools map[string][]string `yaml:",inline"`
}

func ParseToolsConfig(toolsYaml []byte) (ToolsConfig, error) {
	var toolsConfig ToolsConfig
	if err := yaml.Unmarshal(toolsYaml, &toolsConfig); err != nil {
		return ToolsConfig{}, err
	}

	if toolsConfig.ServerTools == nil {
		toolsConfig.ServerTools = make(map[string][]string)
	}

	return toolsConfig, nil
}
