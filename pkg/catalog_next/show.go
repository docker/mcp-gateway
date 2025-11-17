package catalognext

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func Show(ctx context.Context, dao db.DAO, refStr string, format workingset.OutputFormat) error {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse oci-reference %s: %w", refStr, err)
	}

	refStr = oci.FullNameWithoutDigest(ref)

	dbCatalog, err := dao.GetCatalog(ctx, refStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("catalog %s not found", refStr)
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
	return fmt.Sprintf("Reference: %s\nTitle: %s\nSource: %s\nServers:\n%s", catalog.Ref, catalog.Title, catalog.Source, servers)
}
