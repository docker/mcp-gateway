package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/docker/mcp-gateway/pkg/db"
)

func UpdateTools(ctx context.Context, dao db.DAO, id string, addTools, removeTools []string) error {
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

	// Check for overlap between add and remove sets
	addSet := make(map[string]bool)
	for _, toolArg := range addTools {
		addSet[toolArg] = true
	}
	removeSet := make(map[string]bool)
	for _, toolArg := range removeTools {
		removeSet[toolArg] = true
	}

	var overlapping []string
	for tool := range addSet {
		if removeSet[tool] {
			overlapping = append(overlapping, tool)
		}
	}

	addedCount := 0
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
			addedCount++
		}
	}

	removedCount := 0
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
			removedCount++
		}
	}

	err = dao.UpdateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to update working set: %w", err)
	}

	if addedCount == 0 && removedCount == 0 {
		fmt.Printf("No tools were added or removed from working set %s\n", id)
	} else {
		fmt.Printf("Updated working set %s: %d tool(s) added, %d tool(s) removed\n", id, addedCount, removedCount)
	}

	if len(overlapping) > 0 {
		slices.Sort(overlapping)
		fmt.Printf("Warning: The following tool(s) were both added and removed in the same operation: %s\n", strings.Join(overlapping, ", "))
	}

	return nil
}
