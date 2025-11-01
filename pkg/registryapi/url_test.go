package registryapi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseServerURL(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		wantBaseURL     string
		wantAPIVersion  string
		wantServerName  string
		wantVersion     string
		wantVersionsEnd bool
		wantErr         bool
	}{
		{
			name:            "latest version endpoint",
			url:             "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions/latest",
			wantBaseURL:     "https://registry.modelcontextprotocol.io",
			wantAPIVersion:  "v0",
			wantServerName:  "ai.aliengiraffe%2Fspotdb",
			wantVersion:     "latest",
			wantVersionsEnd: true,
			wantErr:         false,
		},
		{
			name:            "versions list endpoint",
			url:             "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions",
			wantBaseURL:     "https://registry.modelcontextprotocol.io",
			wantAPIVersion:  "v0",
			wantServerName:  "ai.aliengiraffe%2Fspotdb",
			wantVersion:     "latest",
			wantVersionsEnd: true,
			wantErr:         false,
		},
		{
			name:            "server base endpoint",
			url:             "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb",
			wantBaseURL:     "https://registry.modelcontextprotocol.io",
			wantAPIVersion:  "v0",
			wantServerName:  "ai.aliengiraffe%2Fspotdb",
			wantVersion:     "latest",
			wantVersionsEnd: false,
			wantErr:         false,
		},
		{
			name:            "specific version endpoint",
			url:             "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions/0.1.0",
			wantBaseURL:     "https://registry.modelcontextprotocol.io",
			wantAPIVersion:  "v0",
			wantServerName:  "ai.aliengiraffe%2Fspotdb",
			wantVersion:     "0.1.0",
			wantVersionsEnd: true,
			wantErr:         false,
		},
		{
			name:            "extra path segments",
			url:             "https://registry.modelcontextprotocol.io/my-cool-registry/for-sure/v0/servers/ai.aliengiraffe%2Fspotdb/versions/latest",
			wantBaseURL:     "https://registry.modelcontextprotocol.io/my-cool-registry/for-sure",
			wantAPIVersion:  "v0",
			wantServerName:  "ai.aliengiraffe%2Fspotdb",
			wantVersion:     "latest",
			wantVersionsEnd: true,
			wantErr:         false,
		},
		{
			name:    "invalid URL - too few segments",
			url:     "https://registry.modelcontextprotocol.io/v0",
			wantErr: true,
		},
		{
			name:    "invalid URL - missing servers path",
			url:     "https://registry.modelcontextprotocol.io/v0/notservers/ai.aliengiraffe%2Fspotdb",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseServerURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.Equal(t, tt.wantBaseURL, got.BaseURL)
			require.Equal(t, tt.wantAPIVersion, got.APIVersion)
			require.Equal(t, tt.wantServerName, got.ServerName)
			require.Equal(t, tt.wantVersion, got.Version)
		})
	}
}

func TestServerURL_String(t *testing.T) {
	tests := []struct {
		name string
		r    *ServerURL
		want string
	}{
		{
			name: "base server URL",
			r: &ServerURL{
				BaseURL:    "https://registry.modelcontextprotocol.io",
				APIVersion: "v0",
				ServerName: "ai.aliengiraffe%2Fspotdb",
			},
			want: "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions/latest",
		},
		{
			name: "latest version endpoint",
			r: &ServerURL{
				BaseURL:    "https://registry.modelcontextprotocol.io",
				APIVersion: "v0",
				ServerName: "ai.aliengiraffe%2Fspotdb",
				Version:    "latest",
			},
			want: "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions/latest",
		},
		{
			name: "specific version endpoint",
			r: &ServerURL{
				BaseURL:    "https://registry.modelcontextprotocol.io",
				APIVersion: "v0",
				ServerName: "ai.aliengiraffe%2Fspotdb",
				Version:    "0.1.0",
			},
			want: "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions/0.1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.r.String())
		})
	}
}

func TestServerURL_HelperMethods(t *testing.T) {
	r := &ServerURL{
		BaseURL:    "https://registry.modelcontextprotocol.io",
		APIVersion: "v0",
		ServerName: "ai.aliengiraffe%2Fspotdb",
		RawURL:     "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb",
	}

	tests := []struct {
		name   string
		method func() string
		want   string
	}{
		{
			name:   "LatestVersionURL",
			method: r.LatestVersionURL,
			want:   "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions/latest",
		},
		{
			name:   "VersionsListURL",
			method: r.VersionsListURL,
			want:   "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions",
		},
		{
			name:   "ServerURL",
			method: r.Raw,
			want:   "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.method())
		})
	}
}

func TestServerURL_IsLatestVersion(t *testing.T) {
	r := &ServerURL{
		Version: "latest",
	}
	require.True(t, r.IsLatestVersion())
	r.Version = "0.1.0"
	require.False(t, r.IsLatestVersion())
	r.Version = ""
	require.True(t, r.IsLatestVersion())
}

func TestServerURL_WithVersion(t *testing.T) {
	tests := []struct {
		name    string
		r       *ServerURL
		version string
		want    string
	}{
		{
			name: "with existing latest version",
			r: &ServerURL{
				BaseURL:    "https://registry.modelcontextprotocol.io",
				APIVersion: "v0",
				ServerName: "ai.aliengiraffe%2Fspotdb",
				Version:    "latest",
				RawURL:     "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb",
			},
			version: "0.1.0",
			want:    "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions/0.1.0",
		},
		{
			name: "without existing version",
			r: &ServerURL{
				BaseURL:    "https://registry.modelcontextprotocol.io",
				APIVersion: "v0",
				ServerName: "ai.aliengiraffe%2Fspotdb",
				RawURL:     "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb",
			},
			version: "0.1.0",
			want:    "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions/0.1.0",
		},
		{
			name: "change to latest version",
			r: &ServerURL{
				BaseURL:    "https://registry.modelcontextprotocol.io",
				APIVersion: "v0",
				ServerName: "ai.aliengiraffe%2Fspotdb",
				Version:    "0.1.0",
				RawURL:     "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb",
			},
			version: "latest",
			want:    "https://registry.modelcontextprotocol.io/v0/servers/ai.aliengiraffe%2Fspotdb/versions/latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.r.WithVersion(tt.version).String())
		})
	}
}
