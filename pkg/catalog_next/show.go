package catalognext

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/goccy/go-yaml"
)

func Show(ctx context.Context, dao db.DAO, digest string, format workingset.OutputFormat) error {
	dbCatalog, err := dao.GetCatalog(ctx, digest)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("catalog %s not found", digest)
		}
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	catalog := NewFromDb(dbCatalog)

	var data []byte
	switch format {
	case workingset.OutputFormatHumanReadable:
		data = []byte(printHumanReadable(catalog))
	case workingset.OutputFormatJSON:
		data, err = json.MarshalIndent(catalog, "", "  ")
	case workingset.OutputFormatYAML:
		data, err = yaml.Marshal(catalog)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal catalog: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

func printHumanReadable(catalog CatalogWithDigest) string {
	servers := ""
	for _, server := range catalog.Servers {
		servers += fmt.Sprintf("  - Type: %s\n", server.Type)
		switch server.Type {
		case workingset.ServerTypeRegistry:
			servers += fmt.Sprintf("    Source: %s\n", server.Source)
		case workingset.ServerTypeImage:
			servers += fmt.Sprintf("    Image: %s\n", server.Image)
		}
	}
	servers = strings.TrimSuffix(servers, "\n")
	return fmt.Sprintf("Digest: %s\nName: %s\nSource: %s\nServers:\n%s", catalog.Digest, catalog.Name, catalog.Source, servers)
}
