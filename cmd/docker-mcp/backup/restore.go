package backup

import (
	"context"
	"encoding/json"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
)

func Restore(_ context.Context, backupData []byte) error {
	var backup Backup
	if err := json.Unmarshal(backupData, &backup); err != nil {
		return err
	}

	if err := config.WriteConfig([]byte(backup.Config)); err != nil {
		return err
	}
	if err := config.WriteRegistry([]byte(backup.Registry)); err != nil {
		return err
	}
	if err := config.WriteTools([]byte(backup.Tools)); err != nil {
		return err
	}

	catalogBefore, err := catalog.ReadConfig()
	if err != nil {
		return err
	}

	if err := config.WriteCatalog([]byte(backup.Catalog)); err != nil {
		return err
	}

	catalogsKeep := map[string]bool{}
	for name, content := range backup.CatalogFiles {
		if err := config.WriteCatalogFile(name, []byte(content)); err != nil {
			return err
		}
		catalogsKeep[name] = true
	}

	for name := range catalogBefore.Catalogs {
		if !catalogsKeep[name] {
			if err := config.RemoveCatalogFile(name); err != nil {
				return err
			}
		}
	}

	return nil
}
