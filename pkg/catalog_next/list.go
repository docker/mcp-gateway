package catalognext

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func List(ctx context.Context, dao db.DAO, format workingset.OutputFormat) error {
	dbCatalogs, err := dao.ListCatalogs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list catalogs: %w", err)
	}

	if len(dbCatalogs) == 0 && format == workingset.OutputFormatHumanReadable {
		fmt.Println("No catalogs found. Use `docker mcp catalog-next create` or `docker mcp catalog-next pull <oci-reference>` to create a catalog.")
		return nil
	}

	catalogs := make([]CatalogWithDigest, len(dbCatalogs))
	for i, dbCatalog := range dbCatalogs {
		catalogs[i] = NewFromDb(&dbCatalog)
	}

	var data []byte
	switch format {
	case workingset.OutputFormatHumanReadable:
		data = []byte(printListHumanReadable(catalogs))
	case workingset.OutputFormatJSON:
		data, err = json.MarshalIndent(catalogs, "", "  ")
	case workingset.OutputFormatYAML:
		data, err = yaml.Marshal(catalogs)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal catalogs: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

func printListHumanReadable(catalogs []CatalogWithDigest) string {
	lines := ""
	for _, catalog := range catalogs {
		lines += fmt.Sprintf("%s\t| %s\t| %s\n", catalog.Ref, catalog.Digest, catalog.Title)
	}
	lines = strings.TrimSuffix(lines, "\n")
	return fmt.Sprintf("Reference | Digest | Title\n%s", lines)
}
