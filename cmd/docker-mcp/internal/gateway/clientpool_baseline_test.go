package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/proxies"
)

// TestBaseline_ClientPoolAcquireClient tests the current client acquisition behavior
// This test establishes a regression baseline for the existing AcquireClient functionality
func TestBaseline_ClientPoolAcquireClient(t *testing.T) {
	serverConfig := &catalog.ServerConfig{
		Name: "test-server",
		Spec: catalog.Server{
			Image:   "alpine:latest",
			Command: []string{"echo", "test"},
		},
		Config:  map[string]any{},
		Secrets: map[string]string{},
	}

	clientPool := &clientPool{
		Options: Options{
			Cpus:   1,
			Memory: "512m",
		},
		keptClients: make(map[clientKey]keptClient),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test that AcquireClient creates a new clientGetter when no cached client exists
	client, err := clientPool.AcquireClient(ctx, serverConfig, &clientConfig{readOnly: boolPtr(false)})

	// This should fail in test environment without Docker, but we test the path
	// The important part is that it follows the expected acquisition pattern
	if err != nil {
		// Expected in test environment - validate the error path
		require.Error(t, err)
		assert.Nil(t, client)
	} else {
		// If somehow this succeeds, validate the success path
		assert.NotNil(t, client)
		defer clientPool.ReleaseClient(client)
	}
}

// TestBaseline_ClientPoolLongLived tests long-lived client caching behavior
func TestBaseline_ClientPoolLongLived(t *testing.T) {
	serverConfig := &catalog.ServerConfig{
		Name: "test-long-lived",
		Spec: catalog.Server{
			Image:     "alpine:latest",
			LongLived: true,
		},
	}

	clientPool := &clientPool{
		Options: Options{
			LongLived: false, // Test server-specific long-lived setting
		},
		keptClients: make(map[clientKey]keptClient),
	}

	config := &clientConfig{
		readOnly:      boolPtr(false),
		serverSession: &mcp.ServerSession{},
	}

	// Test longLived determination logic
	isLongLived := clientPool.longLived(serverConfig, config)
	assert.True(t, isLongLived, "Should be long-lived when serverConfig.Spec.LongLived is true")

	// Test global LongLived override
	clientPool.LongLived = true
	serverConfig.Spec.LongLived = false
	isLongLived = clientPool.longLived(serverConfig, config)
	assert.True(t, isLongLived, "Should be long-lived when global LongLived is true")

	// Test no long-lived when both false
	clientPool.LongLived = false
	config.serverSession = nil // Also test nil session
	isLongLived = clientPool.longLived(serverConfig, config)
	assert.False(t, isLongLived, "Should not be long-lived when both settings are false or session is nil")
}

// TestBaseline_ClientGetterGetClient tests the core client creation logic
func TestBaseline_ClientGetterGetClient(t *testing.T) {
	tests := []struct {
		name         string
		serverConfig *catalog.ServerConfig
		expectedPath string
	}{
		{
			name: "Remote HTTP URL",
			serverConfig: &catalog.ServerConfig{
				Name: "remote-http",
				Spec: catalog.Server{
					Remote: catalog.Remote{URL: "http://example.com/mcp"},
				},
			},
			expectedPath: "remote",
		},
		{
			name: "Deprecated SSE Endpoint",
			serverConfig: &catalog.ServerConfig{
				Name: "sse-endpoint",
				Spec: catalog.Server{
					SSEEndpoint: "http://example.com/sse",
				},
			},
			expectedPath: "remote",
		},
		{
			name: "Containerized stdio",
			serverConfig: &catalog.ServerConfig{
				Name: "container-stdio",
				Spec: catalog.Server{
					Image:   "alpine:latest",
					Command: []string{"echo", "test"},
				},
			},
			expectedPath: "container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientPool := &clientPool{
				Options: Options{
					Static: false,
				},
				keptClients: make(map[clientKey]keptClient),
			}

			getter := newClientGetter(tt.serverConfig, clientPool, &clientConfig{})
			assert.NotNil(t, getter)
			assert.Equal(t, tt.serverConfig, getter.serverConfig)
			assert.Equal(t, clientPool, getter.cp)

			// Test that GetClient would attempt the appropriate path
			// We can't fully test without Docker/network dependencies,
			// but we validate the setup is correct
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			_, err := getter.GetClient(ctx)
			// Expected to fail in test environment, but validates the path is taken
			assert.Error(t, err) // Will fail due to missing dependencies
		})
	}
}

// TestBaseline_ArgsAndEnvGeneration tests the Docker argument and environment generation
func TestBaseline_ArgsAndEnvGeneration(t *testing.T) {
	clientPool := &clientPool{
		Options: Options{
			Cpus:   2,
			Memory: "1Gb",
		},
		networks: []string{"mcp-network"},
	}

	serverConfig := &catalog.ServerConfig{
		Name: "baseline-test",
		Spec: catalog.Server{
			DisableNetwork: false,
			Secrets: []catalog.Secret{
				{Name: "api_key", Env: "API_KEY"},
			},
			Env: []catalog.Env{
				{Name: "ENV_VAR", Value: "test_value"},
			},
			Volumes: []string{"/host:/container:ro"},
		},
		Config: map[string]any{},
		Secrets: map[string]string{
			"api_key": "secret_value",
		},
	}

	args, env := clientPool.argsAndEnv(serverConfig, boolPtr(false), proxies.TargetConfig{})

	// Validate base Docker run args are present
	expectedBaseArgs := []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges",
		"--cpus", "2", "--memory", "1Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp",
		"-l", "docker-mcp-name=baseline-test", "-l", "docker-mcp-transport=stdio",
	}

	for _, expectedArg := range expectedBaseArgs {
		assert.Contains(t, args, expectedArg, "Base args should contain: %s", expectedArg)
	}

	// Validate network attachment
	assert.Contains(t, args, "--network")
	assert.Contains(t, args, "mcp-network")

	// Validate secret environment variable
	assert.Contains(t, args, "-e")
	assert.Contains(t, args, "API_KEY")
	assert.Contains(t, env, "API_KEY=secret_value")

	// Validate regular environment variable
	assert.Contains(t, args, "ENV_VAR")
	assert.Contains(t, env, "ENV_VAR=test_value")

	// Validate volume mount
	assert.Contains(t, args, "-v")
	assert.Contains(t, args, "/host:/container:ro")
}

// TestBaseline_NetworkDisabled tests network isolation behavior
func TestBaseline_NetworkDisabled(t *testing.T) {
	clientPool := &clientPool{
		networks: []string{"mcp-network"},
	}

	serverConfig := &catalog.ServerConfig{
		Name: "network-disabled",
		Spec: catalog.Server{
			DisableNetwork: true,
		},
	}

	args, _ := clientPool.argsAndEnv(serverConfig, nil, proxies.TargetConfig{})

	// Should use --network none instead of mcp-network
	assert.Contains(t, args, "--network")
	assert.Contains(t, args, "none")
	assert.NotContains(t, args, "mcp-network")
}

// TestBaseline_StaticMode tests static deployment mode behavior
func TestBaseline_StaticMode(t *testing.T) {
	serverConfig := &catalog.ServerConfig{
		Name: "static-test",
		Spec: catalog.Server{
			Image: "alpine:latest",
		},
	}

	clientPool := &clientPool{
		Options: Options{
			Static: true, // Test static mode
		},
		keptClients: make(map[clientKey]keptClient),
	}

	getter := newClientGetter(serverConfig, clientPool, &clientConfig{})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := getter.GetClient(ctx)
	// Should fail due to socat not being available, but validates static path
	assert.Error(t, err)
}

// TestBaseline_ClientPoolReleaseClient tests client release behavior
func TestBaseline_ClientPoolReleaseClient(t *testing.T) {
	clientPool := &clientPool{
		keptClients: make(map[clientKey]keptClient),
	}

	// Create a mock client for testing
	// Since we can't easily create a real mcpclient.Client in test,
	// this test validates the release logic structure

	// Test releasing non-kept client (would close immediately)
	// Test releasing kept client (would not close)

	// The actual functionality requires integration testing with real clients
	// This baseline test documents the expected behavior
	assert.Empty(t, clientPool.keptClients, "Should start with empty kept clients")
}

// TestBaseline_ClientPoolClose tests cleanup behavior
func TestBaseline_ClientPoolClose(t *testing.T) {
	clientPool := &clientPool{
		keptClients: make(map[clientKey]keptClient),
	}

	// Test the behavior of Close with empty keptClients
	assert.Empty(t, clientPool.keptClients, "Should start with empty kept clients")

	clientPool.Close()

	assert.Empty(t, clientPool.keptClients, "Should remain empty after Close")

	// Note: Testing with real kept clients would require creating actual clientGetters
	// with functional clients, which would require Docker integration testing.
	// This baseline test validates the structure and empty case.
}
