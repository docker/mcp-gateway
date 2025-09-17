package tools

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/config"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/docker"
)

func Disable(ctx context.Context, docker docker.Client, toolNames []string, serverName string) error {
	return update(ctx, docker, nil, toolNames, serverName)
}

func Enable(ctx context.Context, docker docker.Client, toolNames []string, serverName string) error {
	return update(ctx, docker, toolNames, nil, serverName)
}

func findServerByTool(mcpCatalog catalog.Catalog, toolName string) (string, error) {
	for serverName, server := range mcpCatalog.Servers {
		for _, tool := range server.Tools {
			if tool.Name == toolName {
				return serverName, nil
			}
		}
	}
	return "", fmt.Errorf("tool %q not found in any server", toolName)
}

func validateToolExistsInServer(mcpCatalog catalog.Catalog, serverName string, toolName string) error {
	if _, exists := mcpCatalog.Servers[serverName]; !exists {
		return fmt.Errorf("server %q not found in catalog", serverName)
	}

	for _, tool := range mcpCatalog.Servers[serverName].Tools {
		if tool.Name == toolName {
			return nil
		}
	}
	return fmt.Errorf("tool %q not found in server %q", toolName, serverName)
}

func update(ctx context.Context, docker docker.Client, add []string, remove []string, serverName string) error {
	toolsYAML, err := config.ReadTools(ctx, docker)
	if err != nil {
		return fmt.Errorf("reading tools: %w", err)
	}

	toolsConfig, err := config.ParseToolsConfig(toolsYAML)
	if err != nil {
		return fmt.Errorf("parsing tools: %w", err)
	}

	mcpCatalog, err := catalog.Get(ctx)
	if err != nil {
		return fmt.Errorf("reading catalog: %w", err)
	}

	for _, toolName := range add {
		var targetServerName string

		if serverName != "" {
			if err := validateToolExistsInServer(mcpCatalog, serverName, toolName); err != nil {
				return err
			}
			targetServerName = serverName
		} else {
			discoveredServerName, err := findServerByTool(mcpCatalog, toolName)
			if err != nil {
				return err
			}
			targetServerName = discoveredServerName
		}

		toolAlreadyEnabled := slices.Contains(toolsConfig.ServerTools[targetServerName], toolName)
		if toolAlreadyEnabled {
			continue
		}

		toolsConfig.ServerTools[targetServerName] = append(toolsConfig.ServerTools[targetServerName], toolName)
	}

	for _, toolName := range remove {
		var targetServerName string

		if serverName != "" {
			if err := validateToolExistsInServer(mcpCatalog, serverName, toolName); err != nil {
				return err
			}
			targetServerName = serverName
		} else {
			discoveredServerName, err := findServerByTool(mcpCatalog, toolName)
			if err != nil {
				return err
			}
			targetServerName = discoveredServerName
		}

		if _, exists := toolsConfig.ServerTools[targetServerName]; !exists {
			serverTools := mcpCatalog.Servers[targetServerName].Tools
			var allToolNames []string
			for _, tool := range serverTools {
				allToolNames = append(allToolNames, tool.Name)
			}

			toolsConfig.ServerTools[targetServerName] = []string{}
			for _, tool := range allToolNames {
				if tool != toolName {
					toolsConfig.ServerTools[targetServerName] = append(toolsConfig.ServerTools[targetServerName], tool)
				}
			}
			continue
		}

		tools := toolsConfig.ServerTools[targetServerName]
		newTools := make([]string, 0, len(tools))
		for _, tool := range tools {
			if tool != toolName {
				newTools = append(newTools, tool)
			}
		}
		toolsConfig.ServerTools[targetServerName] = newTools
	}

	for serverName := range toolsConfig.ServerTools {
		sort.Strings(toolsConfig.ServerTools[serverName])
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(toolsConfig.ServerTools); err != nil {
		return fmt.Errorf("encoding tools: %w", err)
	}

	if err := config.WriteTools(buf.Bytes()); err != nil {
		return fmt.Errorf("writing tools: %w", err)
	}

	return nil
}
