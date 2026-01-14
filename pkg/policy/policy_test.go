package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// TestNewDefaultClientNoDesktop verifies Desktop detection can be disabled.
func TestNewDefaultClientNoDesktop(t *testing.T) {
	ctx := desktop.WithNoDockerDesktop(context.Background())
	client := NewDefaultClient(ctx)

	_, ok := client.(NoopClient)
	require.True(t, ok)
}
