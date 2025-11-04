package workingset

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"gopkg.in/yaml.v3"
)

func UpdateConfig(ctx context.Context, dao db.DAO, ociService oci.Service, id string, setConfigArgs []string, getConfigArgs []string, getAll bool, outputFormat OutputFormat) error {
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

	outputMap := make(map[string]string)

	if getAll {
		for _, server := range workingSet.Servers {
			for configName, value := range server.Config {
				outputMap[fmt.Sprintf("%s.%s", server.Snapshot.Server.Name, configName)] = fmt.Sprintf("%v", value)
			}
		}
	} else {
		for _, configArg := range getConfigArgs {
			serverName, configName, found := strings.Cut(configArg, ".")
			if !found {
				return fmt.Errorf("invalid config argument: %s, expected <serverName>.<configName>", configArg)
			}

			server := workingSet.FindServer(serverName)
			if server == nil {
				return fmt.Errorf("server %s not found in working set for argument %s", serverName, configArg)
			}

			if server.Config != nil && server.Config[configName] != nil {
				outputMap[configArg] = fmt.Sprintf("%v", server.Config[configName])
			}
		}
	}

	for _, configArg := range setConfigArgs {
		key, value, found := strings.Cut(configArg, "=")
		if !found {
			return fmt.Errorf("invalid config argument: %s, expected <serverName>.<configName>=<value>", configArg)
		}

		serverName, configName, found := strings.Cut(key, ".")
		if !found {
			return fmt.Errorf("invalid config argument: %s, expected <serverName>.<configName>=<value>", key)
		}

		server := workingSet.FindServer(serverName)
		if server == nil {
			return fmt.Errorf("server %s not found in working set for argument %s", serverName, configArg)
		}

		if server.Config == nil {
			server.Config = make(map[string]any)
		}
		// TODO(cody): validate that schema supports the config we're adding
		server.Config[configName] = value
		outputMap[key] = value
	}

	if len(setConfigArgs) > 0 {
		err := dao.UpdateWorkingSet(ctx, workingSet.ToDb())
		if err != nil {
			return fmt.Errorf("failed to update working set: %w", err)
		}
	}

	switch outputFormat {
	case OutputFormatHumanReadable:
		for configName, value := range outputMap {
			fmt.Printf("%s=%s\n", configName, value)
		}
	case OutputFormatJSON:
		data, err := json.MarshalIndent(outputMap, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}
		fmt.Println(string(data))
	case OutputFormatYAML:
		data, err := yaml.Marshal(outputMap)
		if err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}
		fmt.Println(string(data))
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	return nil
}
