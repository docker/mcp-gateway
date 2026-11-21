package gateway

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/gateway/proxies"
)

func TestIntegrationServerResourceLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a Docker daemon and a local test image")
	}
	image := os.Getenv("MCP_RESOURCE_TEST_IMAGE")
	if image == "" {
		t.Skip("set MCP_RESOURCE_TEST_IMAGE to a local image with sleep, e.g. alpine:3.22")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Do not pull an image as a side effect of running the test suite.
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect", image).CombinedOutput()
	require.NoError(t, err, string(out))
	for _, tc := range []struct {
		name      string
		options   Options
		resources *catalog.Resources
		cpu       int64
		memory    int64
	}{
		{name: "catalog", options: Options{Cpus: 1, Memory: "2g"}, resources: &catalog.Resources{CPUs: "0.25", Memory: "128m"}, cpu: 250_000_000, memory: 128 * 1024 * 1024},
		{name: "operator", options: Options{Cpus: 1, Memory: "2g", ServerCPUs: map[string]string{"worker": "1.5"}, ServerMemory: map[string]string{"worker": "256m"}}, cpu: 1_500_000_000, memory: 256 * 1024 * 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := &clientPool{Options: tc.options}
			args, _, err := cp.argsAndEnv(&catalog.ServerConfig{Name: "worker", Spec: catalog.Server{Image: image, Resources: tc.resources}}, proxies.TargetConfig{})
			require.NoError(t, err)
			args = append(args, "--detach", image, "sleep", "60")
			out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
			require.NoError(t, err, string(out))
			id := strings.TrimSpace(string(out))
			t.Cleanup(func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cleanupCancel()
				output, cleanupErr := exec.CommandContext(cleanupCtx, "docker", "rm", "-f", id).CombinedOutput()
				assert.NoError(t, cleanupErr, string(output))
			})
			out, err = exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .HostConfig}}", id).CombinedOutput()
			require.NoError(t, err, string(out))
			var limits struct {
				NanoCpus int64
				Memory   int64
			}
			require.NoError(t, json.Unmarshal(out, &limits))
			assert.Equal(t, tc.cpu, limits.NanoCpus)
			assert.Equal(t, tc.memory, limits.Memory)
		})
	}
}
