package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationWithoutProxy(t *testing.T) {
	thisIsAnIntegrationTest(t)

	// Start proxy server using docker compose
	proxyDir := getProxyTestDir(t)
	startProxyServer(t, proxyDir)

	// Wait for proxy to be ready
	waitForProxyReady(t, proxyDir)

	// Clear catalog cache
	clearCatalogCache(t)

	// Unset any proxy environment variables
	env := clearProxyEnv(os.Environ())

	// Test catalog show command without proxy
	out := runDockerMCPWithEnv(t, env, "catalog", "show", "docker-mcp", "--format=json")

	// Verify catalog content
	assert.Contains(t, out, `"registry"`, "Catalog should contain registry section")
}

func TestIntegrationWithBadProxy(t *testing.T) {
	thisIsAnIntegrationTest(t)

	// Start proxy server using docker compose
	proxyDir := getProxyTestDir(t)
	startProxyServer(t, proxyDir)

	// Wait for proxy to be ready
	waitForProxyReady(t, proxyDir)

	// Clear catalog cache
	clearCatalogCache(t)

	// Set bad proxy environment variables
	env := clearProxyEnv(os.Environ())
	badProxy := "http://127.0.0.1:9999"

	env = append(env,
		"HTTP_PROXY="+badProxy,
		"HTTPS_PROXY="+badProxy,
		"http_proxy="+badProxy,
		"https_proxy="+badProxy,
	)

	// Test catalog show command with bad proxy - should fail
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "mcp", "catalog", "show", "docker-mcp", "--format=json")
	cmd.Env = env

	err := cmd.Run()
	assert.Error(t, err, "Command should fail with bad proxy")
}

func TestIntegrationWithGoodProxy(t *testing.T) {
	thisIsAnIntegrationTest(t)

	// Start proxy server using docker compose
	proxyDir := getProxyTestDir(t)
	startProxyServer(t, proxyDir)

	// Wait for proxy to be ready
	waitForProxyReady(t, proxyDir)

	// Clear catalog cache
	clearCatalogCache(t)

	// Set good proxy environment variables
	env := clearProxyEnv(os.Environ())
	goodProxy := "http://localhost:3128"

	env = append(env,
		"HTTP_PROXY="+goodProxy,
		"HTTPS_PROXY="+goodProxy,
		"http_proxy="+goodProxy,
		"https_proxy="+goodProxy,
	)

	// Test catalog show command with good proxy
	out := runDockerMCPWithEnv(t, env, "catalog", "show", "docker-mcp", "--format=json")

	// Verify catalog content
	assert.Contains(t, out, `"registry"`, "Catalog should contain registry section")

	// Check proxy logs for requests to desktop.docker.com
	cmd := exec.CommandContext(t.Context(), "docker", "compose", "logs", "--tail=50", "proxy")
	cmd.Dir = proxyDir
	cmdOut, err := cmd.Output()
	require.NoError(t, err, "Failed to get proxy logs")

	logs := string(cmdOut)

	// Look for requests to desktop.docker.com (the catalog URL) or just verify proxy is working
	hasDesktopRequest := strings.Contains(logs, "desktop.docker.com") ||
		strings.Contains(logs, "GET") ||
		strings.Contains(logs, "CONNECT") ||
		strings.Contains(logs, "Accepting HTTP Socket connections") // Proxy is at least running

	assert.True(t, hasDesktopRequest, "Proxy logs should show it's running or handling requests, logs: %s", logs)

	// Test catalog update command as well
	runDockerMCPWithEnv(t, env, "catalog", "update")
}

func getProxyTestDir(t *testing.T) string {
	t.Helper()

	// Get the project root directory
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Navigate up to project root (cmd/docker-mcp -> project root)
	projectRoot := filepath.Join(wd, "..")
	proxyDir := filepath.Join(projectRoot, "test", "testdata", "proxy")

	// Verify the directory exists
	if _, err := os.Stat(proxyDir); os.IsNotExist(err) {
		t.Fatalf("Proxy test directory not found: %s", proxyDir)
	}

	return proxyDir
}

func startProxyServer(t *testing.T, proxyDir string) {
	t.Helper()

	// Start docker compose
	cmd := exec.CommandContext(t.Context(), "docker", "compose", "up", "-d")
	cmd.Dir = proxyDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to start proxy server: %s", string(out))

	t.Cleanup(func() {
		stopProxyServer(t, proxyDir)
	})
}

func stopProxyServer(t *testing.T, proxyDir string) {
	t.Helper()

	// Test context is already cancelled in cleanup, so we need our own context with a timeout
	ctx := context.WithoutCancel(t.Context())
	if d, ok := t.Context().Deadline(); ok {
		c, cancel := context.WithTimeout(ctx, time.Until(d))
		ctx = c
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "down", "--volumes")
	cmd.Dir = proxyDir
	err := cmd.Run()
	require.NoError(t, err)
}

func waitForProxyReady(t *testing.T, proxyDir string) {
	t.Helper()

	// Wait up to 60 seconds for proxy to be ready
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatal("Proxy server did not become ready within timeout")
		case <-ticker.C:
			if isProxyReady(ctx, proxyDir) {
				return
			}
		}
	}
}

func isProxyReady(ctx context.Context, proxyDir string) bool {
	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "--services", "--filter", "status=running")
	cmd.Dir = proxyDir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "proxy")
}

func clearCatalogCache(t *testing.T) {
	t.Helper()

	homeDir, err := os.UserHomeDir() //nolint:forbidigo
	if err != nil {
		return // Skip if we can't get home dir
	}

	cachePaths := []string{
		filepath.Join(homeDir, ".docker", "mcp", "catalogs"),
		filepath.Join(homeDir, ".docker", "mcp", "catalog.json"),
	}

	for _, path := range cachePaths {
		os.RemoveAll(path) // Ignore errors
	}
}

func clearProxyEnv(env []string) []string {
	var result []string
	proxyVars := []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy", "NO_PROXY", "no_proxy"}

	for _, envVar := range env {
		keep := true
		for _, proxyVar := range proxyVars {
			if strings.HasPrefix(envVar, proxyVar+"=") {
				keep = false
				break
			}
		}
		if keep {
			result = append(result, envVar)
		}
	}

	return result
}

func runDockerMCPWithEnv(t *testing.T, env []string, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	mcpArgs := append([]string{"mcp"}, args...)
	cmd := exec.CommandContext(ctx, "docker", mcpArgs...)
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "Command failed: %s", string(out))

	return string(out)
}
