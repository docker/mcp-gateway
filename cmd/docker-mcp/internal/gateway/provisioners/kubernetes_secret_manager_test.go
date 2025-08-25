package provisioners

import (
	"testing"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

// mockConfigResolver implements ConfigResolver for testing
type mockConfigResolver struct {
	secrets map[string]map[string]string // serverName -> {envVar -> value}
}

func (m *mockConfigResolver) ResolveSecrets(serverName string) map[string]string {
	if secrets, exists := m.secrets[serverName]; exists {
		return secrets
	}
	return map[string]string{}
}

func (m *mockConfigResolver) ResolveEnvironment(_ string) map[string]string {
	return map[string]string{} // Not needed for secret tests
}

func (m *mockConfigResolver) ResolveCommand(_ string) []string {
	return []string{} // Not needed for secret tests
}

func TestGatewayKubernetesSecretManager_GetSecretSpecs(t *testing.T) {
	tests := []struct {
		name            string
		serverName      string
		serverConfigs   map[string]*catalog.ServerConfig
		resolvedSecrets map[string]map[string]string
		expectedSpecs   map[string]map[string]string
	}{
		{
			name:       "no secrets",
			serverName: "test-server",
			serverConfigs: map[string]*catalog.ServerConfig{
				"test-server": {
					Name: "test-server",
					Spec: catalog.Server{
						Secrets: []catalog.Secret{}, // No secrets
					},
					Secrets: map[string]string{},
				},
			},
			resolvedSecrets: map[string]map[string]string{},
			expectedSpecs:   map[string]map[string]string{},
		},
		{
			name:       "single secret",
			serverName: "dockerhub-server",
			serverConfigs: map[string]*catalog.ServerConfig{
				"dockerhub-server": {
					Name: "dockerhub-server",
					Spec: catalog.Server{
						Secrets: []catalog.Secret{
							{Name: "dockerhub.username", Env: "DOCKERHUB_USERNAME"},
						},
					},
					Secrets: map[string]string{
						"dockerhub.username": "myusername", // Template resolved
					},
				},
			},
			resolvedSecrets: map[string]map[string]string{
				"dockerhub-server": {
					"DOCKERHUB_USERNAME": "myusername",
				},
			},
			expectedSpecs: map[string]map[string]string{
				"test-secrets": {
					"dockerhub.username": "myusername", // Key is secret.Name, value is resolved
				},
			},
		},
		{
			name:       "multiple secrets",
			serverName: "multi-secret-server",
			serverConfigs: map[string]*catalog.ServerConfig{
				"multi-secret-server": {
					Name: "multi-secret-server",
					Spec: catalog.Server{
						Secrets: []catalog.Secret{
							{Name: "dockerhub.username", Env: "DOCKERHUB_USERNAME"},
							{Name: "dockerhub.token", Env: "DOCKERHUB_TOKEN"},
							{Name: "api.key", Env: "API_KEY"},
						},
					},
					Secrets: map[string]string{
						"dockerhub.username": "myuser",
						"dockerhub.token":    "secret-token",
						"api.key":            "api-secret",
					},
				},
			},
			resolvedSecrets: map[string]map[string]string{
				"multi-secret-server": {
					"DOCKERHUB_USERNAME": "myuser",
					"DOCKERHUB_TOKEN":    "secret-token",
					"API_KEY":            "api-secret",
				},
			},
			expectedSpecs: map[string]map[string]string{
				"test-secrets": {
					"dockerhub.username": "myuser",
					"dockerhub.token":    "secret-token",
					"api.key":            "api-secret",
				},
			},
		},
		{
			name:       "missing secret value",
			serverName: "missing-secret-server",
			serverConfigs: map[string]*catalog.ServerConfig{
				"missing-secret-server": {
					Name: "missing-secret-server",
					Spec: catalog.Server{
						Secrets: []catalog.Secret{
							{Name: "existing.secret", Env: "EXISTING_SECRET"},
							{Name: "missing.secret", Env: "MISSING_SECRET"},
						},
					},
					Secrets: map[string]string{
						"existing.secret": "exists",
						// "missing.secret" not provided
					},
				},
			},
			resolvedSecrets: map[string]map[string]string{
				"missing-secret-server": {
					"EXISTING_SECRET": "exists",
					// MISSING_SECRET not resolved
				},
			},
			expectedSpecs: map[string]map[string]string{
				"test-secrets": {
					"existing.secret": "exists",
					// Only secrets with resolved values are included
				},
			},
		},
		{
			name:            "nonexistent server",
			serverName:      "nonexistent",
			serverConfigs:   map[string]*catalog.ServerConfig{},
			resolvedSecrets: map[string]map[string]string{},
			expectedSpecs:   map[string]map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock config resolver
			mockResolver := &mockConfigResolver{
				secrets: tt.resolvedSecrets,
			}

			// Create secret manager
			secretManager := NewGatewayKubernetesSecretManager(mockResolver, tt.serverConfigs, "test-secrets")

			// Test GetSecretSpecs
			actualSpecs := secretManager.GetSecretSpecs(tt.serverName)

			// Verify results
			if len(actualSpecs) != len(tt.expectedSpecs) {
				t.Errorf("GetSecretSpecs() returned %d specs, expected %d", len(actualSpecs), len(tt.expectedSpecs))
				return
			}

			for expectedSecretName, expectedData := range tt.expectedSpecs {
				actualData, exists := actualSpecs[expectedSecretName]
				if !exists {
					t.Errorf("Expected secret %q not found in results", expectedSecretName)
					continue
				}

				if len(actualData) != len(expectedData) {
					t.Errorf("Secret %q has %d keys, expected %d", expectedSecretName, len(actualData), len(expectedData))
					continue
				}

				for expectedKey, expectedValue := range expectedData {
					actualValue, exists := actualData[expectedKey]
					if !exists {
						t.Errorf("Secret %q missing key %q", expectedSecretName, expectedKey)
						continue
					}
					if actualValue != expectedValue {
						t.Errorf("Secret %q key %q = %q, expected %q", expectedSecretName, expectedKey, actualValue, expectedValue)
					}
				}
			}
		})
	}
}

func TestGatewayKubernetesSecretManager_GetSecretKeyRefs(t *testing.T) {
	tests := []struct {
		name            string
		serverName      string
		serverConfigs   map[string]*catalog.ServerConfig
		expectedKeyRefs map[string]SecretKeyRef
	}{
		{
			name:       "no secrets",
			serverName: "test-server",
			serverConfigs: map[string]*catalog.ServerConfig{
				"test-server": {
					Name: "test-server",
					Spec: catalog.Server{
						Secrets: []catalog.Secret{}, // No secrets
					},
					Secrets: map[string]string{},
				},
			},
			expectedKeyRefs: map[string]SecretKeyRef{},
		},
		{
			name:       "single secret key ref",
			serverName: "dockerhub-server",
			serverConfigs: map[string]*catalog.ServerConfig{
				"dockerhub-server": {
					Name: "dockerhub-server",
					Spec: catalog.Server{
						Secrets: []catalog.Secret{
							{Name: "dockerhub.username", Env: "DOCKERHUB_USERNAME"},
						},
					},
					Secrets: map[string]string{
						"dockerhub.username": "myusername", // Secret exists
					},
				},
			},
			expectedKeyRefs: map[string]SecretKeyRef{
				"DOCKERHUB_USERNAME": {
					Name: "test-secrets",
					Key:  "dockerhub.username",
				},
			},
		},
		{
			name:       "multiple secret key refs",
			serverName: "multi-secret-server",
			serverConfigs: map[string]*catalog.ServerConfig{
				"multi-secret-server": {
					Name: "multi-secret-server",
					Spec: catalog.Server{
						Secrets: []catalog.Secret{
							{Name: "dockerhub.username", Env: "DOCKERHUB_USERNAME"},
							{Name: "dockerhub.token", Env: "DOCKERHUB_TOKEN"},
							{Name: "api.key", Env: "API_KEY"},
						},
					},
					Secrets: map[string]string{
						"dockerhub.username": "user",
						"dockerhub.token":    "token",
						"api.key":            "key",
					},
				},
			},
			expectedKeyRefs: map[string]SecretKeyRef{
				"DOCKERHUB_USERNAME": {
					Name: "test-secrets",
					Key:  "dockerhub.username",
				},
				"DOCKERHUB_TOKEN": {
					Name: "test-secrets",
					Key:  "dockerhub.token",
				},
				"API_KEY": {
					Name: "test-secrets",
					Key:  "api.key",
				},
			},
		},
		{
			name:       "missing secret value excluded",
			serverName: "missing-secret-server",
			serverConfigs: map[string]*catalog.ServerConfig{
				"missing-secret-server": {
					Name: "missing-secret-server",
					Spec: catalog.Server{
						Secrets: []catalog.Secret{
							{Name: "existing.secret", Env: "EXISTING_SECRET"},
							{Name: "missing.secret", Env: "MISSING_SECRET"},
						},
					},
					Secrets: map[string]string{
						"existing.secret": "exists",
						// "missing.secret" not provided - should be excluded
					},
				},
			},
			expectedKeyRefs: map[string]SecretKeyRef{
				"EXISTING_SECRET": {
					Name: "test-secrets",
					Key:  "existing.secret",
				},
				// MISSING_SECRET should not appear since secret value doesn't exist
			},
		},
		{
			name:            "nonexistent server",
			serverName:      "nonexistent",
			serverConfigs:   map[string]*catalog.ServerConfig{},
			expectedKeyRefs: map[string]SecretKeyRef{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock config resolver (doesn't matter for this test)
			mockResolver := &mockConfigResolver{}

			// Create secret manager
			secretManager := NewGatewayKubernetesSecretManager(mockResolver, tt.serverConfigs, "test-secrets")

			// Test GetSecretKeyRefs
			actualKeyRefs := secretManager.GetSecretKeyRefs(tt.serverName)

			// Verify results
			if len(actualKeyRefs) != len(tt.expectedKeyRefs) {
				t.Errorf("GetSecretKeyRefs() returned %d key refs, expected %d", len(actualKeyRefs), len(tt.expectedKeyRefs))
				return
			}

			for expectedEnvVar, expectedKeyRef := range tt.expectedKeyRefs {
				actualKeyRef, exists := actualKeyRefs[expectedEnvVar]
				if !exists {
					t.Errorf("Expected environment variable %q not found in key refs", expectedEnvVar)
					continue
				}

				if actualKeyRef.Name != expectedKeyRef.Name {
					t.Errorf("KeyRef for %q has Name=%q, expected %q", expectedEnvVar, actualKeyRef.Name, expectedKeyRef.Name)
				}
				if actualKeyRef.Key != expectedKeyRef.Key {
					t.Errorf("KeyRef for %q has Key=%q, expected %q", expectedEnvVar, actualKeyRef.Key, expectedKeyRef.Key)
				}
			}
		})
	}
}
