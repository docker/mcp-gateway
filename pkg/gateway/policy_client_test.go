package gateway

import (
	"context"
	"testing"

	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/policy"
)

// TestNewPolicyClient_NoDesktopContextUsesNoop verifies context override returns the noop client.
func TestNewPolicyClient_NoDesktopContextUsesNoop(t *testing.T) {
	ctx := desktop.WithNoDockerDesktop(context.Background())

	client := newPolicyClient(ctx)
	if _, ok := client.(policy.NoopClient); !ok {
		t.Fatalf("expected NoopClient when Desktop detection is disabled")
	}
}
