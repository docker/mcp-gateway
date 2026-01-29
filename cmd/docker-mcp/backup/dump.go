package backup

import (
	"context"
	"encoding/json"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/docker"
)

func Dump(ctx context.Context, docker docker.Client) ([]byte, error) {
	configContent, err := config.ReadConfig(ctx, docker)
	if err != nil {
		return nil, err
	}

	registryContent, err := config.ReadRegistry(ctx, docker)
	if err != nil {
		return nil, err
	}

	catalogContent, err := config.ReadCatalog()
	if err != nil {
		return nil, err
	}

	toolsConfig, err := config.ReadTools(ctx, docker)
	if err != nil {
		return nil, err
	}

	catalogConfig, err := catalog.ReadConfig()
	if err != nil {
		return nil, err
	}

	catalogFiles := make(map[string]string)
	for name := range catalogConfig.Catalogs {
		catalogFileContent, err := config.ReadCatalogFile(name)
		if err != nil {
			return nil, err
		}
		catalogFiles[name] = string(catalogFileContent)
	}

	backup := Backup{
		Config:       string(configContent),
		Registry:     string(registryContent),
		Catalog:      string(catalogContent),
		CatalogFiles: catalogFiles,
		Tools:        string(toolsConfig),
	}

	return json.Marshal(backup)
}
