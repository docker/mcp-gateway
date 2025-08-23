package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/provisioners"
)

// TestBaseline_ConfigurationFind tests the current Find behavior for servers and tools
func TestBaseline_ConfigurationFind(t *testing.T) {
	config := Configuration{
		serverNames: []string{"mcp-server", "poci-server"},
		servers: map[string]catalog.Server{
			"mcp-server": {
				Image: "mcp/test:latest",
			},
			"remote-server": {
				Remote: catalog.Remote{URL: "http://example.com/mcp"},
			},
			"sse-server": {
				SSEEndpoint: "http://example.com/sse",
			},
			"poci-server": {
				Tools: []catalog.Tool{
					{Name: "tool1", Container: catalog.Container{Image: "tool/image:latest"}},
					{Name: "tool2", Container: catalog.Container{Image: "tool/image2:latest"}},
				},
			},
		},
		config: map[string]map[string]any{
			"mcp-server": {"setting": "value"},
		},
		secrets: map[string]string{
			"secret1": "value1",
		},
	}

	// Test MCP server with image
	serverConfig, tools, found := config.Find("mcp-server")
	assert.True(t, found)
	assert.NotNil(t, serverConfig)
	assert.Nil(t, tools)
	assert.Equal(t, "mcp-server", serverConfig.Name)
	assert.Equal(t, "mcp/test:latest", serverConfig.Spec.Image)
	assert.Equal(t, map[string]any{"mcp-server": map[string]any{"setting": "value"}}, serverConfig.Config)
	assert.Equal(t, config.secrets, serverConfig.Secrets)

	// Test remote server
	serverConfig, tools, found = config.Find("remote-server")
	assert.True(t, found)
	assert.NotNil(t, serverConfig)
	assert.Nil(t, tools)
	assert.Equal(t, "http://example.com/mcp", serverConfig.Spec.Remote.URL)

	// Test deprecated SSE server
	serverConfig, tools, found = config.Find("sse-server")
	assert.True(t, found)
	assert.NotNil(t, serverConfig)
	assert.Nil(t, tools)
	assert.Equal(t, "http://example.com/sse", serverConfig.Spec.SSEEndpoint)

	// Test POCI server with tools
	serverConfig, tools, found = config.Find("poci-server")
	assert.True(t, found)
	assert.Nil(t, serverConfig)
	assert.NotNil(t, tools)
	assert.Len(t, *tools, 2)
	assert.Contains(t, *tools, "tool1")
	assert.Contains(t, *tools, "tool2")

	// Test non-existent server
	serverConfig, tools, found = config.Find("non-existent")
	assert.False(t, found)
	assert.Nil(t, serverConfig)
	assert.Nil(t, tools)

	// Test trimming whitespace
	serverConfig, _, found = config.Find(" mcp-server ")
	assert.True(t, found)
	assert.NotNil(t, serverConfig)
	assert.Equal(t, "mcp-server", serverConfig.Name)
}

// TestBaseline_ConfigurationDockerImages tests Docker image collection
func TestBaseline_ConfigurationDockerImages(t *testing.T) {
	config := Configuration{
		serverNames: []string{"mcp-server", "poci-server", "non-existent"},
		servers: map[string]catalog.Server{
			"mcp-server": {
				Image: "mcp/test:latest",
			},
			"poci-server": {
				Tools: []catalog.Tool{
					{Name: "tool1", Container: catalog.Container{Image: "tool/image:latest"}},
					{Name: "tool2", Container: catalog.Container{Image: "tool/image2:latest"}},
				},
			},
		},
	}

	images := config.DockerImages(provisioners.DockerProvisioner)

	// Should be sorted and unique
	expected := []string{"mcp/test:latest", "tool/image2:latest", "tool/image:latest"}
	assert.Equal(t, expected, images)

	// Test Kubernetes mode includes alpine sidecar image
	imagesK8s := config.DockerImages(provisioners.KubernetesProvisioner)
	expectedK8s := []string{"alpine:3.22.1", "mcp/test:latest", "tool/image2:latest", "tool/image:latest"}
	assert.Equal(t, expectedK8s, imagesK8s)
}

// TestBaseline_ConfigurationServerNames tests ServerNames accessor
func TestBaseline_ConfigurationServerNames(t *testing.T) {
	expected := []string{"server1", "server2", "server3"}
	config := Configuration{
		serverNames: expected,
	}

	assert.Equal(t, expected, config.ServerNames())
}

// NOTE: File-based configuration tests with Docker client dependency have been
// intentionally separated to avoid compilation issues. These tests focus on the
// pure configuration logic without Docker client dependencies.
// Integration tests requiring Docker client are in separate test files.
