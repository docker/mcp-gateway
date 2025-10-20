package workingset

import (
	"encoding/json"

	"github.com/docker/mcp-gateway/pkg/config"
)

type Config struct {
	WorkingSets map[string]WorkingSet `json:"workingSets"`
}

type WorkingSet struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Servers     []string `json:"servers"`
}

func ReadConfig() (*Config, error) {
	buf, err := config.ReadWorkingSets()
	if err != nil {
		return nil, err
	}

	var result Config
	if len(buf) > 0 {
		if err := json.Unmarshal(buf, &result); err != nil {
			return nil, err
		}
	}

	if result.WorkingSets == nil {
		result.WorkingSets = map[string]WorkingSet{}
	}

	return &result, nil
}

func WriteConfig(cfg *Config) error {
	if cfg.WorkingSets == nil {
		cfg.WorkingSets = map[string]WorkingSet{}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return config.WriteWorkingSets(data)
}
