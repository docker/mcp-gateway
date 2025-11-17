package catalog

import (
	"context"
	"os"

	"github.com/docker/mcp-gateway/pkg/config"
)

func Reset(ctx context.Context) error {
	catalogsDir, err := config.FilePath("catalogs")
	if err != nil {
		return err
	}
	if err := os.RemoveAll(catalogsDir); err != nil {
		return err
	}

	if err := WriteConfig(&Config{}); err != nil {
		return err
	}

	// Automatically reimport the Docker catalog
	return Import(ctx, DockerCatalogName)
}
