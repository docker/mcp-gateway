package workingset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/db"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
)

func List(ctx context.Context, dao db.DAO, format OutputFormat) error {
	dbSets, err := dao.ListWorkingSets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list profiles: %w", err)
	}

	if len(dbSets) == 0 && format == OutputFormatHumanReadable {
		fmt.Println("No profiles found. Use `docker mcp profile create --name <name>` to create a profile.")
		return nil
	}

	workingSets := make([]WorkingSet, len(dbSets))
	policyClient := policycli.ClientForCLI(ctx)
	showPolicy := policyClient != nil
	for i, dbWorkingSet := range dbSets {
		workingSets[i] = NewFromDb(&dbWorkingSet)
		attachWorkingSetPolicy(ctx, policyClient, &workingSets[i], true)
	}

	var data []byte
	switch format {
	case OutputFormatHumanReadable:
		data = []byte(printListHumanReadable(workingSets, showPolicy))
	case OutputFormatJSON:
		data, err = json.MarshalIndent(workingSets, "", "  ")
	case OutputFormatYAML:
		data, err = yaml.Marshal(workingSets)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal profiles: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

func printListHumanReadable(workingSets []WorkingSet, showPolicy bool) string {
	lines := ""
	for _, workingSet := range workingSets {
		if showPolicy {
			lines += fmt.Sprintf(
				"%s\t%s\t%s\n",
				workingSet.ID,
				workingSet.Name,
				policycli.StatusLabel(workingSet.Policy),
			)
		} else {
			lines += fmt.Sprintf(
				"%s\t%s\n",
				workingSet.ID,
				workingSet.Name,
			)
		}
	}
	lines = strings.TrimSuffix(lines, "\n")
	if showPolicy {
		return fmt.Sprintf("ID\tTitle\tPolicy\n----\t-----\t------\n%s", lines)
	}
	return fmt.Sprintf("ID\tTitle\n----\t-----\n%s", lines)
}
