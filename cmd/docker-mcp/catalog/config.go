package catalog

import (
	"context"
	"encoding/json"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/config"
)

type Config struct {
	Catalogs map[string]Catalog `json:"catalogs"`
}

type Catalog struct {
	DisplayName string `json:"displayName"`
	URL         string `json:"url,omitempty"`
	LastUpdate  string `json:"lastUpdate,omitempty"`
}

func ReadConfig() (*Config, error) {
	buf, err := config.ReadCatalog()
	if err != nil {
		return nil, err
	}

	var result Config
	if len(buf) > 0 {
		if err := json.Unmarshal(buf, &result); err != nil {
			return nil, err
		}
	}

	if result.Catalogs == nil {
		result.Catalogs = map[string]Catalog{}
	}

	return &result, nil
}

func WriteConfig(cfg *Config) error {
	if cfg.Catalogs == nil {
		cfg.Catalogs = map[string]Catalog{}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return config.WriteCatalog(data)
}

func ReadConfigWithDefaultCatalog(ctx context.Context) (*Config, error) {
	cfg, err := ReadConfig()
	if err != nil {
		return nil, err
	}

	if _, found := cfg.Catalogs[DockerCatalogName]; found {
		return cfg, nil
	}

	if err := runImport(ctx, DockerCatalogName); err != nil {
		return nil, err
	}

	return ReadConfig()
}

func ReadCatalogFile(name string) ([]byte, error) {
	return config.ReadCatalogFile(name)
}

func WriteCatalogFile(name string, content []byte) error {
	return config.WriteCatalogFile(name, content)
}
