package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCommunity(t *testing.T) {
	tests := []struct {
		name     string
		server   Server
		expected bool
	}{
		{
			name:     "nil metadata",
			server:   Server{},
			expected: false,
		},
		{
			name:     "empty tags",
			server:   Server{Metadata: &Metadata{Tags: []string{}}},
			expected: false,
		},
		{
			name:     "tags without community",
			server:   Server{Metadata: &Metadata{Tags: []string{"official", "verified"}}},
			expected: false,
		},
		{
			name:     "tags with community",
			server:   Server{Metadata: &Metadata{Tags: []string{"community"}}},
			expected: true,
		},
		{
			name:     "community among other tags",
			server:   Server{Metadata: &Metadata{Tags: []string{"remote", "community", "oauth"}}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.server.IsCommunity())
		})
	}
}

func TestHasExplicitOAuthProviders(t *testing.T) {
	tests := []struct {
		name     string
		server   Server
		expected bool
	}{
		{
			name:     "non-remote server with oauth",
			server:   Server{Type: "server", OAuth: &OAuth{Providers: []OAuthProvider{{Provider: "github"}}}},
			expected: false,
		},
		{
			name:     "remote without oauth",
			server:   Server{Type: "remote"},
			expected: false,
		},
		{
			name:     "remote with nil oauth",
			server:   Server{Type: "remote", OAuth: nil},
			expected: false,
		},
		{
			name:     "remote with empty providers",
			server:   Server{Type: "remote", OAuth: &OAuth{Providers: []OAuthProvider{}}},
			expected: false,
		},
		{
			name:     "remote with oauth providers",
			server:   Server{Type: "remote", OAuth: &OAuth{Providers: []OAuthProvider{{Provider: "github"}}}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.server.HasExplicitOAuthProviders())
		})
	}
}
