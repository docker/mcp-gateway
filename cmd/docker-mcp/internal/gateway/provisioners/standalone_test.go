package provisioners

import (
	"testing"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

// Note: mockConfigResolver is defined in kubernetes_secret_manager_test.go

// Test just the core secret manager functionality without any Kubernetes dependencies
func TestStandaloneSecretManager(t *testing.T) {
	// Test data
	serverConfigs := map[string]*catalog.ServerConfig{
		"test-server": {
			Name: "test-server",
			Spec: catalog.Server{
				Secrets: []catalog.Secret{
					{Name: "dockerhub.username", Env: "DOCKERHUB_USERNAME"},
					{Name: "api.key", Env: "API_KEY"},
				},
			},
			Secrets: map[string]string{
				"dockerhub.username": "testuser",
				"api.key":            "secret123",
			},
		},
	}

	// Mock resolver
	mockResolver := &mockConfigResolver{
		secrets: map[string]map[string]string{
			"test-server": {
				"DOCKERHUB_USERNAME": "testuser",
				"API_KEY":            "secret123",
			},
		},
	}

	// Test secret manager
	secretManager := NewGatewayKubernetesSecretManager(mockResolver, serverConfigs, "mcp-test-server-secrets")

	// Test GetSecretSpecs
	secretSpecs := secretManager.GetSecretSpecs("test-server")
	expectedSecretName := "mcp-test-server-secrets" // This matches the secret name passed to NewGatewayKubernetesSecretManager

	if len(secretSpecs) != 1 {
		t.Fatalf("Expected 1 secret spec, got %d", len(secretSpecs))
	}

	secretData, exists := secretSpecs[expectedSecretName]
	if !exists {
		t.Fatalf("Expected secret %q not found", expectedSecretName)
	}

	expectedData := map[string]string{
		"dockerhub.username": "testuser",
		"api.key":            "secret123",
	}

	if len(secretData) != len(expectedData) {
		t.Errorf("Expected %d secret data entries, got %d", len(expectedData), len(secretData))
	}

	for key, expectedValue := range expectedData {
		actualValue, exists := secretData[key]
		if !exists {
			t.Errorf("Expected secret data key %q not found", key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("Secret data[%q] = %q, expected %q", key, actualValue, expectedValue)
		}
	}

	// Test GetSecretKeyRefs
	keyRefs := secretManager.GetSecretKeyRefs("test-server")
	expectedRefs := map[string]SecretKeyRef{
		"DOCKERHUB_USERNAME": {
			Name: expectedSecretName,
			Key:  "dockerhub.username",
		},
		"API_KEY": {
			Name: expectedSecretName,
			Key:  "api.key",
		},
	}

	if len(keyRefs) != len(expectedRefs) {
		t.Errorf("Expected %d key refs, got %d", len(expectedRefs), len(keyRefs))
	}

	for envVar, expectedRef := range expectedRefs {
		actualRef, exists := keyRefs[envVar]
		if !exists {
			t.Errorf("Expected key ref for %q not found", envVar)
			continue
		}
		if actualRef.Name != expectedRef.Name {
			t.Errorf("KeyRef[%q].Name = %q, expected %q", envVar, actualRef.Name, expectedRef.Name)
		}
		if actualRef.Key != expectedRef.Key {
			t.Errorf("KeyRef[%q].Key = %q, expected %q", envVar, actualRef.Key, expectedRef.Key)
		}
	}

	t.Logf("✓ Secret manager tests passed")
}

func TestStandaloneSecretNaming(t *testing.T) {
	tests := []struct {
		serverName    string
		expectedName  string
		shouldBeValid bool
	}{
		{"simple", "mcp-simple-secrets", true},
		{"my-server", "mcp-my-server-secrets", true},
		{"dockerhub", "mcp-dockerhub-secrets", true},
	}

	for _, tt := range tests {
		t.Run(tt.serverName, func(t *testing.T) {
			// Create minimal config for the server
			serverConfigs := map[string]*catalog.ServerConfig{
				tt.serverName: {
					Name: tt.serverName,
					Spec: catalog.Server{
						Secrets: []catalog.Secret{
							{Name: "test.secret", Env: "TEST_SECRET"},
						},
					},
					Secrets: map[string]string{
						"test.secret": "test-value",
					},
				},
			}

			mockResolver := &mockConfigResolver{
				secrets: map[string]map[string]string{
					tt.serverName: {
						"TEST_SECRET": "test-value",
					},
				},
			}

			secretManager := NewGatewayKubernetesSecretManager(mockResolver, serverConfigs, "mcp-test-server-secrets")
			secretSpecs := secretManager.GetSecretSpecs(tt.serverName)

			if len(secretSpecs) == 0 {
				t.Errorf("Expected secret to be generated")
				return
			}

			var actualSecretName string
			for secretName := range secretSpecs {
				actualSecretName = secretName
				break
			}

			// The secret manager uses the shared secret name passed to constructor
			expectedSharedSecretName := "mcp-test-server-secrets"
			if actualSecretName != expectedSharedSecretName {
				t.Errorf("Expected secret name %q, got %q", expectedSharedSecretName, actualSecretName)
			}
		})
	}
}

func TestStandaloneEdgeCases(t *testing.T) {
	// Test empty server
	emptyConfig := map[string]*catalog.ServerConfig{
		"empty": {
			Name:    "empty",
			Spec:    catalog.Server{Secrets: []catalog.Secret{}},
			Secrets: map[string]string{},
		},
	}

	mockResolver := &mockConfigResolver{}
	secretManager := NewGatewayKubernetesSecretManager(mockResolver, emptyConfig, "mcp-gateway-secrets")

	// Should return empty specs for server with no secrets
	specs := secretManager.GetSecretSpecs("empty")
	if len(specs) != 0 {
		t.Errorf("Expected no secrets for empty server, got %d", len(specs))
	}

	keyRefs := secretManager.GetSecretKeyRefs("empty")
	if len(keyRefs) != 0 {
		t.Errorf("Expected no key refs for empty server, got %d", len(keyRefs))
	}

	// Test nonexistent server
	specs = secretManager.GetSecretSpecs("nonexistent")
	if len(specs) != 0 {
		t.Errorf("Expected no secrets for nonexistent server, got %d", len(specs))
	}

	keyRefs = secretManager.GetSecretKeyRefs("nonexistent")
	if len(keyRefs) != 0 {
		t.Errorf("Expected no key refs for nonexistent server, got %d", len(keyRefs))
	}

	t.Logf("✓ Edge case tests passed")
}
