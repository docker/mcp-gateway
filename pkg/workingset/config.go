package workingset

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
)

func UpdateConfig(ctx context.Context, dao db.DAO, ociService oci.Service, id string, setConfigArgs, getConfigArgs, delConfigArgs []string, getAll bool, outputFormat OutputFormat) error {
	// Verify there is not conflict
	for _, delConfigArg := range delConfigArgs {
		for _, setConfigArg := range setConfigArgs {
			first, _, found := strings.Cut(setConfigArg, "=")
			if found && delConfigArg == first {
				return fmt.Errorf("cannot both delete and set the same config value: %s", delConfigArg)
			}
		}
	}

	dbWorkingSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("profile %s not found", id)
		}
		return fmt.Errorf("failed to get profile: %w", err)
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
				return fmt.Errorf("server %s not found in profile for argument %s", serverName, configArg)
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
			return fmt.Errorf("server %s not found in profile for argument %s", serverName, configArg)
		}

		if server.Config == nil {
			server.Config = make(map[string]any)
		}
		// TODO(cody): validate that schema supports the config we're adding and map it to the right type (right now we're forcing a string)
		server.Config[configName] = value
		outputMap[key] = value
	}

	for _, delConfigArg := range delConfigArgs {
		serverName, configName, found := strings.Cut(delConfigArg, ".")
		if !found {
			return fmt.Errorf("invalid config argument: %s, expected <serverName>.<configName>", delConfigArg)
		}

		server := workingSet.FindServer(serverName)
		if server == nil {
			return fmt.Errorf("server %s not found in working set for argument %s", serverName, delConfigArg)
		}

		if server.Config != nil && server.Config[configName] != nil {
			delete(server.Config, configName)
			delete(outputMap, delConfigArg)
		}
	}

	if len(setConfigArgs) > 0 || len(delConfigArgs) > 0 {
		err := dao.UpdateWorkingSet(ctx, workingSet.ToDb())
		if err != nil {
			return fmt.Errorf("failed to update profile: %w", err)
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
