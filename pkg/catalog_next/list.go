package catalognext

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/policy"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
	policycontext "github.com/docker/mcp-gateway/pkg/policy/context"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func List(ctx context.Context, dao db.DAO, format workingset.OutputFormat) error {
	dbCatalogs, err := dao.ListCatalogs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list catalogs: %w", err)
	}

	if len(dbCatalogs) == 0 && format == workingset.OutputFormatHumanReadable {
		fmt.Println("No catalogs found. Use `docker mcp catalog create` or `docker mcp catalog pull <oci-reference>` to create a catalog.")
		return nil
	}

	summaries := make([]CatalogSummary, len(dbCatalogs))
	policyClient := policycli.ClientForCLI(ctx)

	// Build batch request for all catalog policy evaluations.
	var requests []policy.Request
	for _, dbCatalog := range dbCatalogs {
		requests = append(requests, policycontext.BuildCatalogRequest(
			policyContextForCatalog(dbCatalog.Ref),
			dbCatalog.Ref,
			policy.ActionLoad,
		))
	}

	// Evaluate all requests in a single batch call.
	var decisions []policy.Decision
	if policyClient != nil && len(requests) > 0 {
		decisions, _ = policyClient.EvaluateBatch(ctx, requests)
	}

	// Build summaries with policy decisions.
	for i, dbCatalog := range dbCatalogs {
		summaries[i] = CatalogSummary{
			Ref:    dbCatalog.Ref,
			Digest: dbCatalog.Digest,
			Title:  dbCatalog.Title,
		}
		if i < len(decisions) {
			summaries[i].Policy = decisionToPtr(decisions[i])
		}
	}

	var data []byte
	switch format {
	case workingset.OutputFormatHumanReadable:
		data = []byte(printListHumanReadable(summaries))
	case workingset.OutputFormatJSON:
		data, err = json.MarshalIndent(summaries, "", "  ")
	case workingset.OutputFormatYAML:
		data, err = yaml.Marshal(summaries)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal catalogs: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

func printListHumanReadable(catalogs []CatalogSummary) string {
	lines := ""
	for _, catalog := range catalogs {
		lines += fmt.Sprintf(
			"%s\t| %s\t| %s\t| %s\n",
			catalog.Ref,
			catalog.Digest,
			catalog.Title,
			policycli.StatusLabel(catalog.Policy),
		)
	}
	lines = strings.TrimSuffix(lines, "\n")
	return fmt.Sprintf("Reference | Digest | Title | Policy\n%s", lines)
}
