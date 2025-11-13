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

func UpdateTools(ctx context.Context, dao db.DAO, id string, enable, disable, enableAll, disableAll []string) error {
	if len(enable) == 0 && len(disable) == 0 && len(enableAll) == 0 && len(disableAll) == 0 {
		return fmt.Errorf("must provide at least one flag: --enable, --disable, --enable-all, or --disable-all")
	}
	dbWorkingSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("working set %s not found", id)
		}
		return fmt.Errorf("failed to get working set: %w", err)
	}
	workingSet := NewFromDb(dbWorkingSet)

	// Handle enable-all for specified servers
	enableAllCount := 0
	for _, serverName := range enableAll {
		server := workingSet.FindServer(serverName)
		if server == nil {
			return fmt.Errorf("server %s not found in working set", serverName)
		}
		if server.Tools != nil {
			server.Tools = nil
			enableAllCount++
		}
	}

	// Handle disable-all for specified servers
	disableAllCount := 0
	for _, serverName := range disableAll {
		server := workingSet.FindServer(serverName)
		if server == nil {
			return fmt.Errorf("server %s not found in working set", serverName)
		}
		if server.Tools == nil || len(server.Tools) > 0 {
			server.Tools = []string{}
			disableAllCount++
		}
	}

	// Check for overlap between enable and disable sets
	enableSet := make(map[string]bool)
	for _, toolArg := range enable {
		enableSet[toolArg] = true
	}
	disableSet := make(map[string]bool)
	for _, toolArg := range disable {
		disableSet[toolArg] = true
	}

	var overlapping []string
	for tool := range enableSet {
		if disableSet[tool] {
			overlapping = append(overlapping, tool)
		}
	}

	enabledCount := 0
	for _, toolArg := range enable {
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
			enabledCount++
		}
	}

	disabledCount := 0
	for _, toolArg := range disable {
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
			disabledCount++
		}
	}

	err = dao.UpdateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to update working set: %w", err)
	}

	if enabledCount == 0 && disabledCount == 0 && enableAllCount == 0 && disableAllCount == 0 {
		fmt.Printf("No changes made to working set %s\n", id)
	} else {
		if enableAllCount > 0 {
			fmt.Printf("Enabled all tools for %d server(s) in working set %s\n", enableAllCount, id)
		}
		if disableAllCount > 0 {
			fmt.Printf("Disabled all tools for %d server(s) in working set %s\n", disableAllCount, id)
		}
		if enabledCount > 0 || disabledCount > 0 {
			fmt.Printf("Updated working set %s: %d tool(s) enabled, %d tool(s) disabled\n", id, enabledCount, disabledCount)
		}
	}

	if len(overlapping) > 0 {
		slices.Sort(overlapping)
		fmt.Printf("Warning: The following tool(s) were both enabled and disabled in the same operation: %s\n", strings.Join(overlapping, ", "))
	}

	return nil
}
