package catalognext

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func Show(ctx context.Context, dao db.DAO, ociService oci.Service, refStr string, format workingset.OutputFormat, pullOptionParam string) error {
	pullOption, pullInterval, err := parsePullOption(pullOptionParam)
	if err != nil {
		return err
	}

	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse oci-reference %s: %w", refStr, err)
	}
	if !oci.IsValidInputReference(ref) {
		return fmt.Errorf("reference %s must be a valid OCI reference without a digest", refStr)
	}

	refStr = oci.FullNameWithoutDigest(ref)

	if pullOption == PullOptionAlways {
		fmt.Fprintf(os.Stderr, "Pulling catalog %s...\n", refStr)
		_, err := pullCatalog(ctx, dao, ociService, refStr)
		if err != nil {
			return fmt.Errorf("failed to pull catalog %s: %w", refStr, err)
		}
	}

	dbCatalog, err := dao.GetCatalog(ctx, refStr)
	if err != nil && errors.Is(err, sql.ErrNoRows) && (pullOption == PullOptionMissing || pullOption == PullOptionDuration) {
		fmt.Fprintf(os.Stderr, "Pulling catalog %s (missing)...\n", refStr)
		_, err = pullCatalog(ctx, dao, ociService, refStr)
		if err != nil {
			return fmt.Errorf("failed to pull missing catalog %s: %w", refStr, err)
		}
		// Reload the catalog after pulling
		dbCatalog, err = dao.GetCatalog(ctx, refStr)
		if err != nil {
			return fmt.Errorf("failed to get catalog %s: %w", refStr, err)
		}
	} else if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("catalog %s not found", refStr)
		}
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	if pullOption == PullOptionDuration && dbCatalog.LastUpdated != nil && time.Since(*dbCatalog.LastUpdated) > pullInterval {
		fmt.Fprintf(os.Stderr, "Pulling catalog %s... (last update was %s ago)\n", refStr, time.Since(*dbCatalog.LastUpdated).Round(time.Second))
		_, err := pullCatalog(ctx, dao, ociService, refStr)
		if err != nil {
			return fmt.Errorf("failed to pull catalog %s: %w", refStr, err)
		}
		// Reload the catalog
		dbCatalog, err = dao.GetCatalog(ctx, refStr)
		if err != nil {
			return fmt.Errorf("failed to get catalog %s: %w", refStr, err)
		}
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
		case workingset.ServerTypeRemote:
			servers += fmt.Sprintf("    Endpoint: %s\n", server.Endpoint)
		}
	}
	servers = strings.TrimSuffix(servers, "\n")
	return fmt.Sprintf("Reference: %s\nTitle: %s\nSource: %s\nServers:\n%s", catalog.Ref, catalog.Title, catalog.Source, servers)
}

func parsePullOption(pullOptionParam string) (PullOption, time.Duration, error) {
	if pullOptionParam == "" {
		return PullOptionNever, 0, nil
	}

	var pullOption PullOption
	var pullInterval time.Duration
	isPullOption := slices.Contains(SupportedPullOptions(), pullOptionParam)
	if isPullOption {
		pullOption = PullOption(pullOptionParam)
	} else {
		// Maybe duration
		duration, err := time.ParseDuration(pullOptionParam)
		if err != nil {
			return PullOptionNever, 0, fmt.Errorf("failed to parse pull option %s: should be %s, or duration (e.g. '1h', '1d')", pullOptionParam, strings.Join(SupportedPullOptions(), ", "))
		}
		if duration < 0 {
			return PullOptionNever, 0, fmt.Errorf("duration %s must be positive", duration)
		}
		pullOption = PullOptionDuration
		pullInterval = duration
	}

	return pullOption, pullInterval, nil
}
