package manager

import (
	"testing"
)

func TestSecretMode_String(t *testing.T) {
	tests := []struct {
		mode SecretMode
		want string
	}{
		{ReferenceModeOnly, "reference"},
		{ValueMode, "value"},
		{HybridMode, "hybrid"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if string(tt.mode) != tt.want {
				t.Errorf("got %v, want %v", tt.mode, tt.want)
			}
		})
	}
}

func TestSecretReference_Basic(t *testing.T) {
	ref := SecretReference{
		Name:     "test-secret",
		Provider: "test-provider",
	}

	if ref.Name != "test-secret" {
		t.Errorf("unexpected name: %s", ref.Name)
	}

	if ref.Provider != "test-provider" {
		t.Errorf("unexpected provider: %s", ref.Provider)
	}
}

func TestEnvironmentCapabilities(t *testing.T) {
	caps := &EnvironmentCapabilities{
		HasDockerDesktop:    true,
		HasSwarmMode:        false,
		HasCredentialHelper: false,
		SupportsSecureMount: true,
		RecommendedStrategy: "desktop-label",
	}

	if !caps.HasDockerDesktop {
		t.Error("expected HasDockerDesktop to be true")
	}

	if !caps.SupportsSecureMount {
		t.Error("expected SupportsSecureMount to be true")
	}

	if caps.RecommendedStrategy != "desktop-label" {
		t.Errorf("unexpected recommended strategy: %s", caps.RecommendedStrategy)
	}
}
