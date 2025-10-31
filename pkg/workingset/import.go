package workingset

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
)

func Import(ctx context.Context, dao db.DAO, ociService oci.Service, filename string) error {
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

	// Resolve snapshots for each server before saving
	for i := range len(workingSet.Servers) {
		snapshot, err := resolveSnapshot(ctx, ociService, workingSet.Servers[i])
		if err != nil {
			return fmt.Errorf("failed to resolve snapshot for server[%d]: %w", i, err)
		}
		if snapshot != nil {
			workingSet.Servers[i].Snapshot = snapshot
		}
	}

	if err := workingSet.Validate(); err != nil {
		return fmt.Errorf("invalid working set: %w", err)
	}

	dbSet := workingSet.ToDb()

	_, err = dao.GetWorkingSet(ctx, workingSet.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to get working set: %w", err)
	}

	if err != nil { // Not found
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
