package policyutil

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/policy"
)

// TestAllowedToolCount verifies policy filtering for tool counts.
func TestAllowedToolCount(t *testing.T) {
	t.Helper()

	tools := []catalog.Tool{
		{Name: "a"},
		{Name: "b", Policy: &policy.Decision{Allowed: true}},
		{Name: "c", Policy: &policy.Decision{Allowed: false}},
	}

	require.Equal(t, 2, AllowedToolCount(tools))
}

// TestServerSpecFromSnapshotWithSnapshot verifies snapshot data is used.
func TestServerSpecFromSnapshotWithSnapshot(t *testing.T) {
	t.Helper()

	snapshot := &catalog.Server{Name: "snap", Image: "image:latest"}
	spec, name := ServerSpecFromSnapshot(snapshot, "image", "", "image:latest", "")

	require.Equal(t, "snap", name)
	require.Equal(t, "image:latest", spec.Image)
}

// TestServerSpecFromSnapshotFallback verifies fallback behavior without snapshot.
func TestServerSpecFromSnapshotFallback(t *testing.T) {
	t.Helper()

	spec, name := ServerSpecFromSnapshot(nil, "registry", "mcp/example", "", "")

	require.Equal(t, "mcp/example", name)
	require.Equal(t, "registry", spec.Type)
	require.Equal(t, "mcp/example", spec.Image)
}

// TestFallbackServerName verifies fallback naming per server type.
func TestFallbackServerName(t *testing.T) {
	t.Helper()

	require.Equal(t, "src", FallbackServerName("registry", "src", "", ""))
	require.Equal(t, "img", FallbackServerName("image", "", "img", ""))
	require.Equal(t, "url", FallbackServerName("remote", "", "", "url"))
}
