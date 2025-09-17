package catalog

import (
	"context"
	"os"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/config"
)

func Reset(context.Context) error {
	catalogsDir, err := config.FilePath("catalogs")
	if err != nil {
		return err
	}
	if err := os.RemoveAll(catalogsDir); err != nil {
		return err
	}

	return WriteConfig(&Config{})
}
