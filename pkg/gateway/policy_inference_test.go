package gateway

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

// TestInferServerSourceType verifies policy source type inference behavior.
func TestInferServerSourceType(t *testing.T) {
	tests := []struct {
		name     string
		server   catalog.Server
		expected string
	}{
		{name: "explicit_type", server: catalog.Server{Type: "registry"}, expected: "registry"},
		{name: "remote_url", server: catalog.Server{Remote: catalog.Remote{URL: "https://example.com"}}, expected: "remote"},
		{name: "sse_endpoint", server: catalog.Server{SSEEndpoint: "https://example.com/sse"}, expected: "remote"},
		{name: "image", server: catalog.Server{Image: "mcp/example"}, expected: "image"},
		{name: "empty", server: catalog.Server{}, expected: ""},
		{
			name:     "explicit_type_overrides_remote",
			server:   catalog.Server{Type: "image", Remote: catalog.Remote{URL: "https://example.com"}},
			expected: "image",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := inferServerSourceType(tc.server)
			require.Equal(t, tc.expected, result)
		})
	}
}

// TestInferPolicyServerSource verifies policy server source selection behavior.
func TestInferPolicyServerSource(t *testing.T) {
	tests := []struct {
		name           string
		serverSource   string
		server         catalog.Server
		expectedSource string
	}{
		{
			name:           "registry_uses_image",
			serverSource:   "registry",
			server:         catalog.Server{Image: "mcp/example"},
			expectedSource: "mcp/example",
		},
		{
			name:           "image_uses_image",
			serverSource:   "image",
			server:         catalog.Server{Image: "mcp/example"},
			expectedSource: "mcp/example",
		},
		{
			name:           "remote_uses_sse",
			serverSource:   "remote",
			server:         catalog.Server{SSEEndpoint: "https://example.com/sse"},
			expectedSource: "https://example.com/sse",
		},
		{
			name:           "remote_uses_url",
			serverSource:   "remote",
			server:         catalog.Server{Remote: catalog.Remote{URL: "https://example.com"}},
			expectedSource: "https://example.com",
		},
		{
			name:           "fallback_image",
			serverSource:   "",
			server:         catalog.Server{Image: "mcp/example"},
			expectedSource: "mcp/example",
		},
		{
			name:           "fallback_endpoint",
			serverSource:   "",
			server:         catalog.Server{Remote: catalog.Remote{URL: "https://example.com"}},
			expectedSource: "https://example.com",
		},
		{
			name:           "empty",
			serverSource:   "",
			server:         catalog.Server{},
			expectedSource: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := inferPolicyServerSource(tc.serverSource, tc.server)
			require.Equal(t, tc.expectedSource, result)
		})
	}
}

// TestInferPolicyServerTransportType verifies policy transport inference behavior.
func TestInferPolicyServerTransportType(t *testing.T) {
	tests := []struct {
		name     string
		server   catalog.Server
		expected string
	}{
		{
			name:     "sse_endpoint",
			server:   catalog.Server{SSEEndpoint: "https://example.com/sse"},
			expected: "sse",
		},
		{
			name:     "remote_transport_sse",
			server:   catalog.Server{Remote: catalog.Remote{Transport: "sse"}},
			expected: "sse",
		},
		{
			name:     "remote_transport_http",
			server:   catalog.Server{Remote: catalog.Remote{Transport: "http"}},
			expected: "streamable",
		},
		{
			name:     "remote_url",
			server:   catalog.Server{Remote: catalog.Remote{URL: "https://example.com"}},
			expected: "streamable",
		},
		{
			name:     "image_stdio",
			server:   catalog.Server{Image: "mcp/example"},
			expected: "stdio",
		},
		{
			name:     "empty",
			server:   catalog.Server{},
			expected: "",
		},
		{
			name: "sse_endpoint_precedence",
			server: catalog.Server{
				SSEEndpoint: "https://example.com/sse",
				Remote:      catalog.Remote{Transport: "http"},
			},
			expected: "sse",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := inferPolicyServerTransportType(tc.server)
			require.Equal(t, tc.expected, result)
		})
	}
}

// TestInferPolicyServerEndpoint verifies endpoint inference behavior.
func TestInferPolicyServerEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		server   catalog.Server
		expected string
	}{
		{
			name:     "sse_endpoint",
			server:   catalog.Server{SSEEndpoint: "https://example.com/sse"},
			expected: "https://example.com/sse",
		},
		{
			name:     "remote_url",
			server:   catalog.Server{Remote: catalog.Remote{URL: "https://example.com"}},
			expected: "https://example.com",
		},
		{
			name:     "empty",
			server:   catalog.Server{},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := inferPolicyServerEndpoint(tc.server)
			require.Equal(t, tc.expected, result)
		})
	}
}
