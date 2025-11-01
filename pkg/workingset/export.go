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
)

func Export(ctx context.Context, dao db.DAO, id string, filename string) error {
	dbSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("working set %s not found", id)
		}
		return fmt.Errorf("failed to get working set: %w", err)
	}

	workingSet := NewFromDb(dbSet)

	// Don't export the snapshots
	for i := range len(workingSet.Servers) {
		workingSet.Servers[i].Snapshot = nil
	}

	var data []byte
	if strings.HasSuffix(strings.ToLower(filename), ".yaml") {
		data, err = yaml.Marshal(workingSet)
	} else if strings.HasSuffix(strings.ToLower(filename), ".json") {
		data, err = json.MarshalIndent(workingSet, "", "  ")
	} else {
		return fmt.Errorf("unsupported file extension: %s, must be .yaml or .json", filename)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal working set: %w", err)
	}

	err = os.WriteFile(filename, data, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write working set: %w", err)
	}

	fmt.Printf("Exported working set %s to %s\n", id, filename)

	return nil
}
