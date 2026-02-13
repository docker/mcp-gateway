package catalognext

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func Show(ctx context.Context, dao db.DAO, ociService oci.Service, refStr string, format workingset.OutputFormat, pullOptionParam string, yqExpr string) error {
	resolved, err := resolveCatalogRef(refStr)
	if err != nil {
		return err
	}
	refStr = resolved

	dbCatalog, err := showWithStaleness(ctx, dao, ociService, refStr, pullOptionParam)
	if err != nil {
		return err
	}

	catalog := NewFromDb(dbCatalog)
	policyClient := policycli.ClientForCLI(ctx)
	attachCatalogPolicy(ctx, policyClient, catalog.Ref, &catalog, true)

	var data []byte
	switch format {
	case workingset.OutputFormatJSON:
		data, err = json.MarshalIndent(catalog, "", "  ")
	case workingset.OutputFormatYAML, workingset.OutputFormatHumanReadable:
		data, err = yaml.Marshal(catalog)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal catalog: %w", err)
	}

	if yqExpr != "" {
		data, err = workingset.ApplyYqExpression(data, format, yqExpr)
		if err != nil {
			return err // wrapping error here would be redundant
		}
	}

	fmt.Println(string(data))

	return nil
}

// showWithStaleness handles the pull-on-stale logic for both OCI and community catalogs.
func showWithStaleness(ctx context.Context, dao db.DAO, ociService oci.Service, refStr string, pullOptionParam string) (*db.Catalog, error) {
	pulledPreviously, err := dao.CheckPullRecord(ctx, refStr)
	if err != nil {
		return nil, fmt.Errorf("failed to check pull record: %w", err)
	}
	pullOptionEvaluator, err := NewPullOptionEvaluator(pullOptionParam, pulledPreviously)
	if err != nil {
		return nil, err
	}

	pulled := false

	if pullOptionEvaluator.IsAlways() {
		fmt.Fprintf(os.Stderr, "Pulling catalog %s...\n", refStr)
		if err := pullCatalog(ctx, dao, ociService, refStr); err != nil {
			return nil, fmt.Errorf("failed to pull catalog %s: %w", refStr, err)
		}
		pulled = true
	}

	dbCatalog, err := dao.GetCatalog(ctx, refStr)
	if err != nil && errors.Is(err, sql.ErrNoRows) && !pulled && pullOptionEvaluator.Evaluate(nil) {
		fmt.Fprintf(os.Stderr, "Pulling catalog %s...\n", refStr)
		if err = pullCatalog(ctx, dao, ociService, refStr); err != nil {
			return nil, fmt.Errorf("failed to pull catalog %s: %w", refStr, err)
		}
		pulled = true
	} else if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("catalog %s not found", refStr)
		}
		return nil, fmt.Errorf("failed to get catalog: %w", err)
	}

	if !pulled && pullOptionEvaluator.Evaluate(dbCatalog) {
		if dbCatalog == nil || dbCatalog.LastUpdated == nil {
			fmt.Fprintf(os.Stderr, "Pulling catalog %s...\n", refStr)
		} else {
			fmt.Fprintf(os.Stderr, "Pulling catalog %s... (last update was %s ago)\n", refStr, time.Since(*dbCatalog.LastUpdated).Round(time.Second))
		}
		if err := pullCatalog(ctx, dao, ociService, refStr); err != nil {
			return nil, fmt.Errorf("failed to pull catalog %s: %w", refStr, err)
		}
		pulled = true
	}

	if pulled {
		dbCatalog, err = dao.GetCatalog(ctx, refStr)
		if err != nil {
			return nil, fmt.Errorf("failed to get catalog %s: %w", refStr, err)
		}
	}

	return dbCatalog, nil
}
