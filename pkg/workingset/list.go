package workingset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/db"
)

func List(ctx context.Context, dao db.DAO, format OutputFormat) error {
	dbSets, err := dao.ListWorkingSets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list working sets: %w", err)
	}

	if len(dbSets) == 0 {
		fmt.Println("No working sets found. Use `docker mcp workingset create --name <name>` to create a working set.")
		return nil
	}

	workingSets := make([]WorkingSet, len(dbSets))
	for i, dbWorkingSet := range dbSets {
		workingSets[i] = NewFromDb(&dbWorkingSet)
	}

	var data []byte
	switch format {
	case OutputFormatHumanReadable:
		data = []byte(printListHumanReadable(workingSets))
	case OutputFormatJSON:
		data, err = json.MarshalIndent(workingSets, "", "  ")
	case OutputFormatYAML:
		data, err = yaml.Marshal(workingSets)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal working sets: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

func printListHumanReadable(workingSets []WorkingSet) string {
	lines := ""
	for _, workingSet := range workingSets {
		lines += fmt.Sprintf("%s\t%s\n", workingSet.ID, workingSet.Name)
	}
	lines = strings.TrimSuffix(lines, "\n")
	return fmt.Sprintf("ID\tName\n----\t----\n%s", lines)
}
