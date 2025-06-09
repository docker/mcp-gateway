package catalog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	configDir   = "mcp"
	configFile  = "catalog.json"
	catalogsDir = "catalogs"
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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	configFilePath := filepath.Join(homeDir, ".docker", configDir, configFile)
	data, err := os.ReadFile(configFilePath)
	if os.IsNotExist(err) {
		return &Config{Catalogs: map[string]Catalog{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var result Config
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	if result.Catalogs == nil {
		result.Catalogs = map[string]Catalog{}
	}

	return &result, nil
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
	file, err := toCatalogFilePath(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(file)
}

func WriteCatalogFile(name string, content []byte) error {
	file, err := toCatalogFilePath(name)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, content, 0o644)
}

func assertConfigDirsExist() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(homeDir, ".docker", configDir, catalogsDir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	return nil
}

func writeConfig(cfg *Config) error {
	if cfg.Catalogs == nil {
		cfg.Catalogs = map[string]Catalog{}
	}

	if err := assertConfigDirsExist(); err != nil {
		return err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configFilePath := filepath.Join(homeDir, ".docker", configDir, configFile)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configFilePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(configFilePath, data, 0o644)
}
