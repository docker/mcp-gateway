package workingset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"gopkg.in/yaml.v3"
)

func Show(ctx context.Context, dao db.DAO, id string, format OutputFormat) error {
	dbSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get working set: %w", err)
	}
	if dbSet == nil {
		return fmt.Errorf("working set %s not found", id)
	}

	workingSet := NewFromDb(dbSet)

	var data []byte
	switch format {
	case OutputFormatJSON:
		data, err = json.MarshalIndent(workingSet, "", "  ")
	case OutputFormatYAML:
		data, err = yaml.Marshal(workingSet)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal working set: %w", err)
	}

	fmt.Println(string(data))

	return nil
}
