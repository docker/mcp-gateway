package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
)

func UpdateTools(ctx context.Context, dao db.DAO, ociService oci.Service, id string, addTools, removeTools []string) error {
	if len(addTools) == 0 && len(removeTools) == 0 {
		return fmt.Errorf("must provide a flag either --add or --remove")
	}
	dbWorkingSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("working set %s not found", id)
		}
		return fmt.Errorf("failed to get working set: %w", err)
	}
	workingSet := NewFromDb(dbWorkingSet)
	if err := workingSet.EnsureSnapshotsResolved(ctx, ociService); err != nil {
		return fmt.Errorf("failed to resolve snapshots: %w", err)
	}
	for _, toolArg := range addTools {
		serverName, toolName, found := strings.Cut(toolArg, ".")
		if !found {
			return fmt.Errorf("invalid tool argument: %s, expected <serverName>.<toolName>", toolArg)
		}
		server := workingSet.FindServer(serverName)
		if server == nil {
			return fmt.Errorf("server %s not found in working set for argument %s", serverName, toolArg)
		}
		if !slices.Contains(server.Tools, toolName) {
			server.Tools = append(server.Tools, toolName)
		}
	}
	for _, toolArg := range removeTools {
		serverName, toolName, found := strings.Cut(toolArg, ".")
		if !found {
			return fmt.Errorf("invalid tool argument: %s, expected <serverName>.<toolName>", toolArg)
		}
		server := workingSet.FindServer(serverName)
		if server == nil {
			return fmt.Errorf("server %s not found in working set for argument %s", serverName, toolArg)
		}
		if idx := slices.Index(server.Tools, toolName); idx != -1 {
			server.Tools = slices.Delete(server.Tools, idx, idx+1)
		}
	}
	err = dao.UpdateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to update working set: %w", err)
	}
	return nil
}
