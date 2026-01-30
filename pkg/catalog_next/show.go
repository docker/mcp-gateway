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
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func Show(ctx context.Context, dao db.DAO, ociService oci.Service, refStr string, format workingset.OutputFormat, pullOptionParam string, yqExpr string) error {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse oci-reference %s: %w", refStr, err)
	}
	if !oci.IsValidInputReference(ref) {
		return fmt.Errorf("reference %s must be a valid OCI reference without a digest", refStr)
	}

	refStr = oci.FullNameWithoutDigest(ref)

	pulledPreviously, err := dao.CheckPullRecord(ctx, refStr)
	if err != nil {
		return fmt.Errorf("failed to check pull record: %w", err)
	}
	pullOptionEvaluator, err := NewPullOptionEvaluator(pullOptionParam, pulledPreviously)
	if err != nil {
		return err // avoid wrapping error for clarity
	}

	pulled := false

	if pullOptionEvaluator.IsAlways() {
		fmt.Fprintf(os.Stderr, "Pulling catalog %s...\n", refStr)
		if err := pullCatalog(ctx, dao, ociService, refStr); err != nil {
			return fmt.Errorf("failed to pull catalog %s: %w", refStr, err)
		}
		pulled = true
	}

	dbCatalog, err := dao.GetCatalog(ctx, refStr)
	if err != nil && errors.Is(err, sql.ErrNoRows) && !pulled && pullOptionEvaluator.Evaluate(nil) {
		fmt.Fprintf(os.Stderr, "Pulling catalog %s...\n", refStr)
		if err = pullCatalog(ctx, dao, ociService, refStr); err != nil {
			return fmt.Errorf("failed to pull catalog %s: %w", refStr, err)
		}
		pulled = true
	} else if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("catalog %s not found", refStr)
		}
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	if !pulled && pullOptionEvaluator.Evaluate(dbCatalog) {
		if dbCatalog == nil || dbCatalog.LastUpdated == nil {
			fmt.Fprintf(os.Stderr, "Pulling catalog %s...\n", refStr)
		} else {
			fmt.Fprintf(os.Stderr, "Pulling catalog %s... (last update was %s ago)\n", refStr, time.Since(*dbCatalog.LastUpdated).Round(time.Second))
		}
		if err := pullCatalog(ctx, dao, ociService, refStr); err != nil {
			return fmt.Errorf("failed to pull catalog %s: %w", refStr, err)
		}
		pulled = true
	}

	if pulled {
		// Reload the catalog after pulling
		dbCatalog, err = dao.GetCatalog(ctx, refStr)
		if err != nil {
			return fmt.Errorf("failed to get catalog %s: %w", refStr, err)
		}
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
