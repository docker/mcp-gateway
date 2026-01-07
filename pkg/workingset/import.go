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
		return fmt.Errorf("failed to read profile file: %w", err)
	}

	var workingSet WorkingSet
	if strings.HasSuffix(strings.ToLower(filename), ".yaml") || strings.HasSuffix(strings.ToLower(filename), ".yml") {
		if err := yaml.Unmarshal(workingSetBuf, &workingSet); err != nil {
			return fmt.Errorf("failed to unmarshal profile: %w", err)
		}
	} else if strings.HasSuffix(strings.ToLower(filename), ".json") {
		if err := json.Unmarshal(workingSetBuf, &workingSet); err != nil {
			return fmt.Errorf("failed to unmarshal profile: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported file extension: %s, must be .yaml or .json", filename)
	}

	// Resolve snapshots for each server before saving
	for i := range len(workingSet.Servers) {
		if workingSet.Servers[i].Snapshot == nil {
			snapshot, err := ResolveSnapshot(ctx, ociService, workingSet.Servers[i])
			if err != nil {
				return fmt.Errorf("failed to resolve snapshot for server[%d]: %w", i, err)
			}
			workingSet.Servers[i].Snapshot = snapshot
		}
	}

	RegisterOAuthProvidersForServers(ctx, workingSet.Servers)

	if err := workingSet.Validate(); err != nil {
		return fmt.Errorf("invalid profile: %w", err)
	}

	dbSet := workingSet.ToDb()

	_, err = dao.GetWorkingSet(ctx, workingSet.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	if err != nil { // Not found
		err = dao.CreateWorkingSet(ctx, dbSet)
		if err != nil {
			return fmt.Errorf("failed to create profile: %w", err)
		}
	} else {
		err = dao.UpdateWorkingSet(ctx, dbSet)
		if err != nil {
			return fmt.Errorf("failed to update profile: %w", err)
		}
	}

	fmt.Printf("Imported profile %s\n", workingSet.ID)

	return nil
}
