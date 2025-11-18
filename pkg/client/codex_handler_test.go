package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCodexConfig_WithProfile(t *testing.T) {
	config := make(map[string]any)

	config["mcp_servers"] = map[string]any{
		"MCP_DOCKER": map[string]any{
			"command": "docker",
			"args":    []any{"mcp", "gateway", "run", "--profile", "test-profile"},
		},
	}

	if mcpServers, ok := config["mcp_servers"].(map[string]any); ok {
		if dockerMCP, exists := mcpServers[DockerMCPCatalog]; exists && dockerMCP != nil {
			if serverConfigMap, ok := dockerMCP.(map[string]any); ok {
				serverConfig := MCPServerSTDIO{
					Name: DockerMCPCatalog,
				}

				if command, ok := serverConfigMap["command"].(string); ok {
					serverConfig.Command = command
				}

				if args, ok := serverConfigMap["args"].([]any); ok {
					for _, arg := range args {
						if argStr, ok := arg.(string); ok {
							serverConfig.Args = append(serverConfig.Args, argStr)
						}
					}
				}

				workingSet := serverConfig.GetWorkingSet()

				assert.Equal(t, "test-profile", workingSet, "WorkingSet should be extracted from config args")
				assert.Equal(t, "docker", serverConfig.Command, "Command should be 'docker'")
				assert.Equal(t, []string{"mcp", "gateway", "run", "--profile", "test-profile"}, serverConfig.Args, "Args should include profile")
			}
		}
	}
}

func TestReadCodexConfig_WithoutProfile(t *testing.T) {
	config := make(map[string]any)
	config["mcp_servers"] = map[string]any{
		"MCP_DOCKER": map[string]any{
			"command": "docker",
			"args":    []any{"mcp", "gateway", "run"},
		},
	}

	if mcpServers, ok := config["mcp_servers"].(map[string]any); ok {
		if dockerMCP, exists := mcpServers[DockerMCPCatalog]; exists && dockerMCP != nil {
			if serverConfigMap, ok := dockerMCP.(map[string]any); ok {
				serverConfig := MCPServerSTDIO{
					Name: DockerMCPCatalog,
				}

				if command, ok := serverConfigMap["command"].(string); ok {
					serverConfig.Command = command
				}

				if args, ok := serverConfigMap["args"].([]any); ok {
					for _, arg := range args {
						if argStr, ok := arg.(string); ok {
							serverConfig.Args = append(serverConfig.Args, argStr)
						}
					}
				}

				workingSet := serverConfig.GetWorkingSet()

				assert.Empty(t, workingSet, "WorkingSet should be empty when no profile in args")
				assert.Equal(t, "docker", serverConfig.Command, "Command should be 'docker'")
				assert.Equal(t, []string{"mcp", "gateway", "run"}, serverConfig.Args, "Args should not include profile")
			}
		}
	}
}

func TestConnectCodex_GeneratesCorrectConfig(t *testing.T) {
	testCases := []struct {
		name         string
		workingSet   string
		expectedWS   string
		expectedArgs []string
	}{
		{
			name:         "with profile",
			workingSet:   "test-profile",
			expectedWS:   "test-profile",
			expectedArgs: []string{"mcp", "gateway", "run", "--profile", "test-profile"},
		},
		{
			name:         "without profile",
			workingSet:   "",
			expectedWS:   "",
			expectedArgs: []string{"mcp", "gateway", "run"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			command := "docker"
			args := []string{"mcp", "gateway", "run"}
			if tc.workingSet != "" {
				args = append(args, "--profile", tc.workingSet)
			}

			config := map[string]any{
				"mcp_servers": map[string]any{
					DockerMCPCatalog: MCPServerConfig{
						Command: command,
						Args:    args,
					},
				},
			}

			if mcpServers, ok := config["mcp_servers"].(map[string]any); ok {
				if dockerMCP, exists := mcpServers[DockerMCPCatalog]; exists && dockerMCP != nil {
					serverCfg := dockerMCP.(MCPServerConfig)

					serverConfig := MCPServerSTDIO{
						Name:    DockerMCPCatalog,
						Command: serverCfg.Command,
						Args:    serverCfg.Args,
					}

					workingSet := serverConfig.GetWorkingSet()

					assert.Equal(t, tc.expectedWS, workingSet, "WorkingSet should match expected")
					assert.Equal(t, tc.expectedArgs, serverConfig.Args, "Args should match expected")
				}
			}
		})
	}
}

func TestCodexConfigExtraction(t *testing.T) {
	configAfterTomlUnmarshal := map[string]any{
		"mcp_servers": map[string]any{
			"MCP_DOCKER": map[string]any{
				"command": "docker",
				"args":    []any{"mcp", "gateway", "run", "--profile", "my-profile"},
			},
		},
	}

	if mcpServers, ok := configAfterTomlUnmarshal["mcp_servers"].(map[string]any); ok {
		dockerMCP, exists := mcpServers["MCP_DOCKER"]
		require.True(t, exists, "MCP_DOCKER should exist")
		require.NotNil(t, dockerMCP, "dockerMCP should not be nil")

		serverConfigMap, ok := dockerMCP.(map[string]any)
		require.True(t, ok, "should be able to cast to map[string]any")

		serverConfig := MCPServerSTDIO{Name: "MCP_DOCKER"}

		if command, ok := serverConfigMap["command"].(string); ok {
			serverConfig.Command = command
		}

		if args, ok := serverConfigMap["args"].([]any); ok {
			for _, arg := range args {
				if argStr, ok := arg.(string); ok {
					serverConfig.Args = append(serverConfig.Args, argStr)
				}
			}
		}

		workingSet := serverConfig.GetWorkingSet()

		assert.Equal(t, "docker", serverConfig.Command)
		assert.Equal(t, []string{"mcp", "gateway", "run", "--profile", "my-profile"}, serverConfig.Args)
		assert.Equal(t, "my-profile", workingSet)
	}
}
