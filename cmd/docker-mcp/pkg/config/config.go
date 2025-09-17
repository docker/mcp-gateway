package config

import "gopkg.in/yaml.v3"

func ParseConfig(configYaml []byte) (map[string]map[string]any, error) {
	var config map[string]map[string]any
	if err := yaml.Unmarshal(configYaml, &config); err != nil {
		return nil, err
	}

	return config, nil
}
