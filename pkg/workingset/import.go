package workingset

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/docker/mcp-gateway/pkg/db"
	"gopkg.in/yaml.v3"
)

func Import(ctx context.Context, dao db.DAO, filename string) error {
	workingSetBuf, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read working set file: %w", err)
	}

	var workingSet WorkingSet
	if strings.HasSuffix(strings.ToLower(filename), ".yaml") {
		if err := yaml.Unmarshal(workingSetBuf, &workingSet); err != nil {
			return fmt.Errorf("failed to unmarshal working set: %w", err)
		}
	} else if strings.HasSuffix(strings.ToLower(filename), ".json") {
		if err := json.Unmarshal(workingSetBuf, &workingSet); err != nil {
			return fmt.Errorf("failed to unmarshal working set: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported file extension: %s, must be .yaml or .json", filename)
	}

	if err := workingSet.Validate(); err != nil {
		return fmt.Errorf("invalid working set: %w", err)
	}

	dbSet := workingSet.ToDb()

	existingSet, err := dao.GetWorkingSet(ctx, workingSet.ID)
	if err != nil {
		return fmt.Errorf("failed to get working set: %w", err)
	}

	if existingSet == nil {
		err = dao.CreateWorkingSet(ctx, dbSet)
		if err != nil {
			return fmt.Errorf("failed to create working set: %w", err)
		}
	} else {
		err = dao.UpdateWorkingSet(ctx, dbSet)
		if err != nil {
			return fmt.Errorf("failed to update working set: %w", err)
		}
	}

	fmt.Printf("Imported working set %s\n", workingSet.ID)

	return nil
}
