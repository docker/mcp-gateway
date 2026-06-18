package mcp

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandEnvIncludesProcessEnvironment(t *testing.T) {
	t.Setenv("MCP_GATEWAY_STDIO_TEST_ENV", "base")

	env := commandEnv([]string{
		"MCP_GATEWAY_STDIO_TEST_EXTRA=extra",
		"MCP_GATEWAY_STDIO_TEST_ENV=override",
	})

	assert.NotEmpty(t, envValue(env, "PATH"))
	assert.Equal(t, "extra", envValue(env, "MCP_GATEWAY_STDIO_TEST_EXTRA"))
	assert.Equal(t, "override", envValue(env, "MCP_GATEWAY_STDIO_TEST_ENV"))
	assert.Contains(t, os.Environ(), "MCP_GATEWAY_STDIO_TEST_ENV=base")
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return strings.TrimPrefix(env[i], prefix)
		}
	}
	return ""
}

func TestStdioClientInitializeAndListTools(t *testing.T) {
	// Skip if running in CI or if Docker is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a stdio client that runs a test MCP server
	// You should replace these with your actual test image and command
	client := NewStdioCmdClient(
		"test-server",
		"docker",
		[]string{"BRAVE_API_KEY=test_key_for_testing"}, // env vars - provide required API key
		"run", "--rm", "-i",
		"-e", "BRAVE_API_KEY",
		"mcp/brave-search@sha256:e13f4693a3421e2b316c8b6196c5c543c77281f9d8938850681e3613bba95115", // Replace with your test image
		// Add any additional command args here if needed
	)

	// Test initialization
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	initParams := &mcp.InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: &mcp.Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	err := client.Initialize(ctx, initParams, true, nil, nil, nil) // verbose = true for debugging
	require.NoError(t, err, "Failed to initialize stdio client")

	// Test ListTools
	toolsCtx, toolsCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer toolsCancel()

	tools, err := client.Session().ListTools(toolsCtx, &mcp.ListToolsParams{})
	require.NoError(t, err, "Failed to list tools")
	require.NotNil(t, tools, "Tools result should not be nil")
	require.NotNil(t, tools.Tools, "Tools array should not be nil")

	t.Logf("Successfully retrieved %d tools", len(tools.Tools))

	// Basic assertions about tools
	for i, tool := range tools.Tools {
		assert.NotEmpty(t, tool.Name, "Tool %d should have a name", i)
		assert.NotNil(t, tool.InputSchema, "Tool %d should have an input schema", i)
		t.Logf("Tool %d: %s - %s", i, tool.Name, tool.Description)
	}

	// Clean up
	err = client.Session().Close()
	assert.NoError(t, err, "Failed to close client")
}
