package catalog

import (
	"testing"
)

func TestParseNodeVersion(t *testing.T) {
	tests := []struct {
		name        string
		enginesNode string
		want        string
	}{
		{"empty string", "", ""},
		{"greater than or equal", ">=18", "18"},
		{"greater than or equal with patch", ">=16.17.0", "16"},
		{"greater than or equal with minor", ">=20.0", "20"},
		{"caret constraint", "^18", "18"},
		{"caret with patch", "^20.0.0", "20"},
		{"tilde constraint", "~18", "18"},
		{"tilde with minor", "~20.11", "20"},
		{"greater than", ">16", "16"},
		{"range with upper bound", ">=18 <22", "18"},
		{"or range", "18 || 20 || 22", ""},
		{"garbage input", "foobar", ""},
		{"just a number", "18", "18"},
		{"with spaces", ">= 18", "18"},
		{"complex range", ">=18.0.0 <25.0.0", "18"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNodeVersion(tt.enginesNode)
			if got != tt.want {
				t.Errorf("parseNodeVersion(%q) = %q, want %q", tt.enginesNode, got, tt.want)
			}
		})
	}
}

func TestNodeVersionToImageTag(t *testing.T) {
	tests := []struct {
		name        string
		nodeVersion string
		want        string
	}{
		{"empty defaults to 22", "", "node:22-bookworm-slim"},
		{"node 18", "18", "node:18-bookworm-slim"},
		{"node 20", "20", "node:20-bookworm-slim"},
		{"node 22", "22", "node:22-bookworm-slim"},
		{"node 24", "24", "node:24-bookworm-slim"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeVersionToImageTag(tt.nodeVersion)
			if got != tt.want {
				t.Errorf("nodeVersionToImageTag(%q) = %q, want %q", tt.nodeVersion, got, tt.want)
			}
		})
	}
}
