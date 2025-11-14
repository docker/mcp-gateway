package oauth

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Pure Function Tests (Zero Dependencies)
// ============================================================================

func TestParseDockerConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid osxkeychain",
			input: `{"credsStore": "osxkeychain"}`,
			want:  "osxkeychain",
		},
		{
			name:  "valid desktop",
			input: `{"credsStore": "desktop"}`,
			want:  "desktop",
		},
		{
			name:  "empty credsStore",
			input: `{"credsStore": ""}`,
			want:  "",
		},
		{
			name:  "missing credsStore",
			input: `{}`,
			want:  "",
		},
		{
			name:    "invalid JSON",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseDockerConfig([]byte(tt.input))

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, config.CredsStore)
		})
	}
}

func TestValidateCredsStore(t *testing.T) {
	tests := []struct {
		name       string
		credsStore string
		wantErr    bool
	}{
		{"osxkeychain", "osxkeychain", false},
		{"desktop", "desktop", false},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCredsStore(tt.credsStore)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHelperBinaryName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"osxkeychain", "docker-credential-osxkeychain"},
		{"desktop", "docker-credential-desktop"},
		{"pass", "docker-credential-pass"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, HelperBinaryName(tt.input))
		})
	}
}

// ============================================================================
// Mocks
// ============================================================================

type mockConfigReader struct {
	data []byte
	err  error
}

func (m mockConfigReader) ReadConfig() ([]byte, error) {
	return m.data, m.err
}

type mockCommandChecker struct {
	commands map[string]bool
}

func (m mockCommandChecker) CommandExists(cmd string) bool {
	return m.commands[cmd]
}

type mockModeDetector struct {
	ceMode bool
}

func (m mockModeDetector) IsCEMode() bool {
	return m.ceMode
}

// ============================================================================
// Resolver Tests
// ============================================================================

func TestResolver_DesktopMode_BinaryExists(t *testing.T) {
	resolver := &Resolver{
		ConfigReader: mockConfigReader{},
		CommandChecker: mockCommandChecker{
			commands: map[string]bool{
				"docker-credential-desktop": true,
			},
		},
		ModeDetector: mockModeDetector{ceMode: false},
	}

	result, err := resolver.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "desktop", result)
}

func TestResolver_DesktopMode_NoBinary_FallbackToConfig(t *testing.T) {
	resolver := &Resolver{
		ConfigReader: mockConfigReader{
			data: []byte(`{"credsStore": "osxkeychain"}`),
		},
		CommandChecker: mockCommandChecker{
			commands: map[string]bool{
				"docker-credential-osxkeychain": true,
			},
		},
		ModeDetector: mockModeDetector{ceMode: false},
	}

	result, err := resolver.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "osxkeychain", result)
}

func TestResolver_CEMode_FromConfig(t *testing.T) {
	resolver := &Resolver{
		ConfigReader: mockConfigReader{
			data: []byte(`{"credsStore": "osxkeychain"}`),
		},
		CommandChecker: mockCommandChecker{
			commands: map[string]bool{
				"docker-credential-osxkeychain": true,
			},
		},
		ModeDetector: mockModeDetector{ceMode: true},
	}

	result, err := resolver.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "osxkeychain", result)
}

func TestResolver_ConfigNotFound(t *testing.T) {
	resolver := &Resolver{
		ConfigReader: mockConfigReader{
			err: os.ErrNotExist,
		},
		CommandChecker: mockCommandChecker{},
		ModeDetector:   mockModeDetector{ceMode: true},
	}

	result, err := resolver.Resolve()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Docker config found")
	assert.Empty(t, result)
}

func TestResolver_ReadConfigError(t *testing.T) {
	resolver := &Resolver{
		ConfigReader: mockConfigReader{
			err: fmt.Errorf("permission denied"),
		},
		CommandChecker: mockCommandChecker{},
		ModeDetector:   mockModeDetector{ceMode: true},
	}

	result, err := resolver.Resolve()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
	assert.Empty(t, result)
}

func TestResolver_InvalidJSON(t *testing.T) {
	resolver := &Resolver{
		ConfigReader: mockConfigReader{
			data: []byte(`{invalid`),
		},
		CommandChecker: mockCommandChecker{},
		ModeDetector:   mockModeDetector{ceMode: true},
	}

	result, err := resolver.Resolve()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
	assert.Empty(t, result)
}

func TestResolver_EmptyCredsStore(t *testing.T) {
	resolver := &Resolver{
		ConfigReader: mockConfigReader{
			data: []byte(`{"credsStore": ""}`),
		},
		CommandChecker: mockCommandChecker{},
		ModeDetector:   mockModeDetector{ceMode: true},
	}

	result, err := resolver.Resolve()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config")
	assert.Empty(t, result)
}

func TestResolver_BinaryNotFound(t *testing.T) {
	resolver := &Resolver{
		ConfigReader: mockConfigReader{
			data: []byte(`{"credsStore": "osxkeychain"}`),
		},
		CommandChecker: mockCommandChecker{
			commands: map[string]bool{}, // No binaries exist
		},
		ModeDetector: mockModeDetector{ceMode: true},
	}

	result, err := resolver.Resolve()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "binary not found")
	assert.Contains(t, err.Error(), "docker-credential-osxkeychain")
	assert.Empty(t, result)
}

func TestResolver_MultipleScenarios(t *testing.T) {
	tests := []struct {
		name           string
		ceMode         bool
		configData     []byte
		configErr      error
		commands       map[string]bool
		wantHelper     string
		wantErr        bool
		wantErrContain string
	}{
		{
			name:   "desktop mode with binary",
			ceMode: false,
			commands: map[string]bool{
				"docker-credential-desktop": true,
			},
			wantHelper: "desktop",
		},
		{
			name:       "ce mode with valid config",
			ceMode:     true,
			configData: []byte(`{"credsStore": "pass"}`),
			commands: map[string]bool{
				"docker-credential-pass": true,
			},
			wantHelper: "pass",
		},
		{
			name:           "no config file",
			ceMode:         true,
			configErr:      os.ErrNotExist,
			wantErr:        true,
			wantErrContain: "no Docker config",
		},
		{
			name:           "corrupt config",
			ceMode:         true,
			configData:     []byte(`{bad json`),
			wantErr:        true,
			wantErrContain: "parse config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &Resolver{
				ConfigReader: mockConfigReader{
					data: tt.configData,
					err:  tt.configErr,
				},
				CommandChecker: mockCommandChecker{
					commands: tt.commands,
				},
				ModeDetector: mockModeDetector{
					ceMode: tt.ceMode,
				},
			}

			result, err := resolver.Resolve()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContain)
				assert.Empty(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantHelper, result)
			}
		})
	}
}
