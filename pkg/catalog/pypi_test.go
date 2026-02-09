package catalog

import (
	"testing"
)

func TestParsePythonVersion(t *testing.T) {
	tests := []struct {
		name           string
		requiresPython string
		want           string
	}{
		{"empty string", "", ""},
		{"greater than or equal uses latest", ">=3.10", ""},
		{"greater than or equal with upper bound uses latest", ">=3.10,<4", ""},
		{"compatible release pins", "~=3.10", "3.10"},
		{"exact version pins", "==3.12", "3.12"},
		{"gte with patch uses latest", ">=3.10.2", ""},
		{"gte with spaces uses latest", ">= 3.10", ""},
		{"garbage input", "foobar", ""},
		{"just a number", "3.10", ""},
		{"greater than only", ">3.10", ""},
		{"multiple gte constraints uses latest", ">=3.8,!=3.9.0,<4.0", ""},
		{"compatible release with patch", "~=3.12.1", "3.12"},
		{"exact with patch", "==3.11.5", "3.11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePythonVersion(tt.requiresPython)
			if got != tt.want {
				t.Errorf("parsePythonVersion(%q) = %q, want %q", tt.requiresPython, got, tt.want)
			}
		})
	}
}

func TestPythonVersionToImageTag(t *testing.T) {
	tests := []struct {
		name          string
		pythonVersion string
		want          string
	}{
		{"empty defaults to bookworm", "", "ghcr.io/astral-sh/uv:python3.14-bookworm-slim"},
		{"3.10 uses bookworm", "3.10", "ghcr.io/astral-sh/uv:python3.10-bookworm-slim"},
		{"3.12 uses bookworm", "3.12", "ghcr.io/astral-sh/uv:python3.12-bookworm-slim"},
		{"3.13 uses bookworm", "3.13", "ghcr.io/astral-sh/uv:python3.13-bookworm-slim"},
		{"3.14 uses bookworm", "3.14", "ghcr.io/astral-sh/uv:python3.14-bookworm-slim"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pythonVersionToImageTag(tt.pythonVersion)
			if got != tt.want {
				t.Errorf("pythonVersionToImageTag(%q) = %q, want %q", tt.pythonVersion, got, tt.want)
			}
		})
	}
}
