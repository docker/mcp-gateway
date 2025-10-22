package workingset

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/docker/mcp-gateway/pkg/config"
)

type Config struct {
	WorkingSets map[string]WorkingSetMetadata `json:"workingSets"`
}

type WorkingSetMetadata struct {
	DisplayName string `json:"displayName"`
}

type WorkingSet struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Servers     []string `json:"servers"`
}

func ReadConfig() (*Config, error) {
	buf, err := config.ReadWorkingSetConfig()
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
		result.WorkingSets = map[string]WorkingSetMetadata{}
	}

	return &result, nil
}

func WriteConfig(cfg *Config) error {
	if cfg.WorkingSets == nil {
		cfg.WorkingSets = map[string]WorkingSetMetadata{}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return config.WriteWorkingSetConfig(data)
}

func ReadWorkingSetFile(name string) (*WorkingSet, error) {
	buf, err := config.ReadWorkingSetFile(name)
	if err != nil {
		return nil, err
	}

	if len(buf) == 0 {
		return nil, os.ErrNotExist
	}

	var ws WorkingSet
	if err := json.Unmarshal(buf, &ws); err != nil {
		return nil, err
	}

	return &ws, nil
}

func WriteWorkingSetFile(name string, ws *WorkingSet) error {
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}

	return config.WriteWorkingSetFile(name, data)
}

func ListWorkingSets() (map[string]WorkingSet, error) {
	cfg, err := ReadConfig()
	if err != nil {
		return nil, err
	}

	workingSets := make(map[string]WorkingSet)
	for name := range cfg.WorkingSets {
		ws, err := ReadWorkingSetFile(name)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Skip missing files
			}
			return nil, err
		}
		workingSets[name] = *ws
	}

	return workingSets, nil
}

func WorkingSetFilePath(name string) (string, error) {
	return config.FilePath(filepath.Join("working-sets", name+".json"))
}
