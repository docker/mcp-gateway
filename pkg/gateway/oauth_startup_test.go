package gateway

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/log"
)

func TestStartOAuthProvidersContainerCE(t *testing.T) {
	for _, explicitCE := range []string{"", "true"} {
		t.Run("CE="+explicitCE, func(t *testing.T) {
			t.Setenv("DOCKER_MCP_IN_CONTAINER", "1")
			t.Setenv("DOCKER_MCP_USE_CE", explicitCE)
			t.Setenv("DOCKER_CONFIG", t.TempDir())
			for _, enabled := range []bool{false, true} {
				var output bytes.Buffer
				log.SetLogWriter(&output)
				t.Cleanup(func() { log.SetLogWriter(os.Stderr) })
				g := &Gateway{Options: Options{McpOAuthDcrEnabled: enabled}}
				// Empty configuration exercises startup without remote servers or credentials.
				require.NoError(t, g.startOAuthProviders(context.Background(), Configuration{}))
				assert.Equal(t, enabled, bytes.Contains(output.Bytes(), []byte("Starting OAuth provider loops")))
				assert.NotContains(t, output.String(), "Starting OAuth notification monitor")
			}
		})
	}
}
