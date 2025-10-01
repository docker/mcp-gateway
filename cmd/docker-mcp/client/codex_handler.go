package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/docker/mcp-gateway/pkg/user"
)

var installCheckPaths = []string{
	"$HOME/.codex",
	"$USERPROFILE\\.codex",
}

// isCodexInstalled checks if the codex binary is installed and working
func isCodexInstalled(_ context.Context) bool {
	for _, path := range installCheckPaths {
		_, err := os.Stat(os.ExpandEnv(path))
		if err == nil {
			return true
		}
	}
	return false
}

// getCodexSetup returns the configuration status for Codex
func getCodexSetup(ctx context.Context) MCPClientCfg {
	result := MCPClientCfg{
		MCPClientCfgBase: MCPClientCfgBase{
			DisplayName:           "Codex",
			Source:                "https://openai.com/codex/",
			Icon:                  "https://www.svgrepo.com/show/306500/openai.svg",
			ConfigName:            vendorCodex,
			Err:                   nil,
			IsMCPCatalogConnected: false,
		},
		IsInstalled:   isCodexInstalled(ctx),
		IsOsSupported: true,
	}

	// If Codex is not installed, return early
	if !result.IsInstalled {
		return result
	}

	// Check if docker mcp gateway is configured in codex config.toml
	config, err := readCodexConfig()
	if err != nil {
		result.Err = classifyError(err)
		return result
	}

	// Check if mcp_servers.DOCKER_MCP exists
	if mcpServers, ok := config["mcp_servers"].(map[string]any); ok {
		if dockerMCP, exists := mcpServers[DockerMCPCatalog]; exists && dockerMCP != nil {
			result.IsMCPCatalogConnected = true
			result.cfg = &MCPJSONLists{STDIOServers: []MCPServerSTDIO{{Name: DockerMCPCatalog}}}
		}
	}

	return result
}

// getCodexConfigPath returns the path to the Codex config file
func getCodexConfigPath() (string, error) {
	home, err := user.HomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

// MCPServerConfig represents the structure of an MCP server in config.toml
type MCPServerConfig struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
}

// readCodexConfig reads and parses the Codex config.toml file
func readCodexConfig() (map[string]any, error) {
	configPath, err := getCodexConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]any
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else {
		config = make(map[string]any)
	}

	return config, nil
}

// writeCodexConfig writes the config back to the Codex config.toml file
func writeCodexConfig(config map[string]any) error {
	configPath, err := getCodexConfigPath()
	if err != nil {
		return err
	}

	output, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, output, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// connectCodex configures docker mcp gateway in Codex by editing config.toml
func connectCodex(_ context.Context) error {
	config, err := readCodexConfig()
	if err != nil {
		return err
	}

	// Ensure mcp_servers section exists
	mcpServers, ok := config["mcp_servers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
		config["mcp_servers"] = mcpServers
	}

	// Add DOCKER_MCP entry
	mcpServers[DockerMCPCatalog] = MCPServerConfig{
		Command: "docker",
		Args:    []string{"mcp", "gateway", "run"},
	}

	return writeCodexConfig(config)
}

// disconnectCodex removes docker mcp gateway from Codex by editing config.toml
func disconnectCodex(_ context.Context) error {
	config, err := readCodexConfig()
	if err != nil {
		return err
	}

	// Remove DOCKER_MCP entry from mcp_servers
	if mcpServers, ok := config["mcp_servers"].(map[string]any); ok {
		delete(mcpServers, DockerMCPCatalog)

		// If mcp_servers is now empty, remove the section
		if len(mcpServers) == 0 {
			delete(config, "mcp_servers")
		}
	}

	return writeCodexConfig(config)
}
