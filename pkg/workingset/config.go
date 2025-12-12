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

	outputMap := make(map[string]any)

	if getAll {
		for _, server := range workingSet.Servers {
			for configName, value := range flattenConfig(server.Config) {
				outputMap[fmt.Sprintf("%s.%s", server.Snapshot.Server.Name, configName)] = value
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

			value := getConfigValue(configName, server.Config)
			if value != nil {
				outputMap[configArg] = value
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
		// TODO(cody): validate that schema supports the config we're adding
		finalValue := any(value)
		var decoded any
		if err := json.Unmarshal([]byte(value), &decoded); err == nil {
			finalValue = decoded
		}
		mergeValueIntoMap(server.Config, configName, finalValue)
		outputMap[key] = finalValue
	}

	for _, delConfigArg := range delConfigArgs {
		serverName, configName, found := strings.Cut(delConfigArg, ".")
		if !found {
			return fmt.Errorf("invalid config argument: %s, expected <serverName>.<configName>", delConfigArg)
		}

		server := workingSet.FindServer(serverName)
		if server == nil {
			return fmt.Errorf("server %s not found in profile for argument %s", serverName, delConfigArg)
		}

		if server.Config != nil && deleteValueFromMap(server.Config, configName) {
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
			fmt.Printf("%s=%v\n", configName, value)
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

func flattenConfig(config map[string]any) map[string]any {
	output := make(map[string]any)
	for key, value := range config {
		if sub, ok := value.(map[string]any); ok {
			for subKey, subValue := range flattenConfig(sub) {
				output[fmt.Sprintf("%s.%s", key, subKey)] = subValue
			}
		} else {
			if strings.Contains(key, ".") {
				// Ignore keys that contain a dot, unsupported
				continue
			}
			output[key] = value
		}
	}
	return output
}

func getConfigValue(configName string, config map[string]any) any {
	if config == nil {
		return nil
	}

	key, rest, foundSep := strings.Cut(configName, ".")
	if !foundSep {
		return config[key]
	}

	childConfig, ok := config[key].(map[string]any)
	if !ok {
		return nil
	}

	return getConfigValue(rest, childConfig)
}

func mergeValueIntoMap(output map[string]any, path string, value any) {
	key, rest, foundSep := strings.Cut(path, ".")
	if !foundSep {
		output[key] = value
		return
	}

	_, found := output[key]
	if !found {
		output[key] = make(map[string]any)
	}

	childConfig, ok := output[key].(map[string]any)
	if !ok {
		return
	}

	mergeValueIntoMap(childConfig, rest, value)
}

func deleteValueFromMap(output map[string]any, path string) bool {
	key, rest, foundSep := strings.Cut(path, ".")
	if !foundSep {
		_, found := output[key]
		delete(output, key)
		return found
	}

	childConfig, ok := output[key].(map[string]any)
	if !ok {
		return false
	}

	deleted := deleteValueFromMap(childConfig, rest)
	if deleted && len(childConfig) == 0 {
		// If child config is now empty, delete the whole object
		delete(output, key)
	}
	return deleted
}
