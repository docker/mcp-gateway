package provisioners

import (
	"strings"
	"testing"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

func TestSecretNaming_KubernetesCompliance(t *testing.T) {
	tests := []struct {
		name          string
		serverName    string
		expectedName  string
		shouldBeValid bool
	}{
		{
			name:          "simple server name",
			serverName:    "myserver",
			expectedName:  "mcp-myserver-secrets",
			shouldBeValid: true,
		},
		{
			name:          "server with hyphens",
			serverName:    "my-cool-server",
			expectedName:  "mcp-my-cool-server-secrets",
			shouldBeValid: true,
		},
		{
			name:          "server with underscores",
			serverName:    "my_server",
			expectedName:  "mcp-my_server-secrets",
			shouldBeValid: false, // Kubernetes secrets can't have underscores
		},
		{
			name:          "long server name",
			serverName:    "very-long-server-name-that-might-cause-issues-with-kubernetes-naming-limits",
			expectedName:  "mcp-very-long-server-name-that-might-cause-issues-with-kubernetes-naming-limits-secrets",
			shouldBeValid: false, // Too long for Kubernetes (>253 chars)
		},
		{
			name:          "server with dots",
			serverName:    "my.server",
			expectedName:  "mcp-my.server-secrets",
			shouldBeValid: false, // Kubernetes secrets can't have dots
		},
		{
			name:          "dockerhub server",
			serverName:    "dockerhub",
			expectedName:  "mcp-dockerhub-secrets",
			shouldBeValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock secret manager to test naming
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

			secretManager := NewGatewayKubernetesSecretManager(mockResolver, serverConfigs, "test-secrets")
			secretSpecs := secretManager.GetSecretSpecs(tt.serverName)

			// Check if secret was generated with expected name
			if len(secretSpecs) == 0 {
				t.Errorf("Expected secret to be generated, but got none")
				return
			}

			var actualSecretName string
			for secretName := range secretSpecs {
				actualSecretName = secretName
				break // Should only be one secret
			}

			// The secret manager uses the shared secret name, not server-specific names
			expectedSharedSecretName := "test-secrets"
			if actualSecretName != expectedSharedSecretName {
				t.Errorf("Expected secret name %q, got %q", expectedSharedSecretName, actualSecretName)
			}

			// Check Kubernetes DNS-1123 compliance for the actual shared secret name
			isValid := isValidKubernetesSecretName(actualSecretName)
			if !isValid {
				t.Errorf("Shared secret name %q should be valid, but got valid=%t", actualSecretName, isValid)
			}
		})
	}
}

func TestSecretKeyRef_Structure(t *testing.T) {
	tests := []struct {
		name         string
		serverName   string
		secrets      []catalog.Secret
		secretValues map[string]string
		expectedRefs map[string]SecretKeyRef
	}{
		{
			name:       "single secret",
			serverName: "test-server",
			secrets: []catalog.Secret{
				{Name: "api.key", Env: "API_KEY"},
			},
			secretValues: map[string]string{
				"api.key": "secret-value",
			},
			expectedRefs: map[string]SecretKeyRef{
				"API_KEY": {
					Name: "test-secrets",
					Key:  "api.key",
				},
			},
		},
		{
			name:       "multiple secrets same resource",
			serverName: "multi-server",
			secrets: []catalog.Secret{
				{Name: "db.username", Env: "DB_USERNAME"},
				{Name: "db.password", Env: "DB_PASSWORD"},
				{Name: "api.token", Env: "API_TOKEN"},
			},
			secretValues: map[string]string{
				"db.username": "dbuser",
				"db.password": "dbpass",
				"api.token":   "token123",
			},
			expectedRefs: map[string]SecretKeyRef{
				"DB_USERNAME": {
					Name: "test-secrets",
					Key:  "db.username",
				},
				"DB_PASSWORD": {
					Name: "test-secrets",
					Key:  "db.password",
				},
				"API_TOKEN": {
					Name: "test-secrets",
					Key:  "api.token",
				},
			},
		},
		{
			name:       "secret name with special characters",
			serverName: "special-server",
			secrets: []catalog.Secret{
				{Name: "dockerhub.username", Env: "DOCKERHUB_USERNAME"},
				{Name: "oauth2.client-id", Env: "OAUTH_CLIENT_ID"},
				{Name: "api_key", Env: "API_KEY"},
			},
			secretValues: map[string]string{
				"dockerhub.username": "user",
				"oauth2.client-id":   "client123",
				"api_key":            "key456",
			},
			expectedRefs: map[string]SecretKeyRef{
				"DOCKERHUB_USERNAME": {
					Name: "test-secrets",
					Key:  "dockerhub.username", // Original secret name preserved in key
				},
				"OAUTH_CLIENT_ID": {
					Name: "test-secrets",
					Key:  "oauth2.client-id",
				},
				"API_KEY": {
					Name: "test-secrets",
					Key:  "api_key",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverConfigs := map[string]*catalog.ServerConfig{
				tt.serverName: {
					Name:    tt.serverName,
					Spec:    catalog.Server{Secrets: tt.secrets},
					Secrets: tt.secretValues,
				},
			}

			mockResolver := &mockConfigResolver{}
			secretManager := NewGatewayKubernetesSecretManager(mockResolver, serverConfigs, "test-secrets")
			actualRefs := secretManager.GetSecretKeyRefs(tt.serverName)

			// Verify all expected refs exist with correct structure
			for expectedEnvVar, expectedRef := range tt.expectedRefs {
				actualRef, exists := actualRefs[expectedEnvVar]
				if !exists {
					t.Errorf("Expected secretKeyRef for environment variable %q not found", expectedEnvVar)
					continue
				}

				if actualRef.Name != expectedRef.Name {
					t.Errorf("SecretKeyRef[%q].Name = %q, expected %q", expectedEnvVar, actualRef.Name, expectedRef.Name)
				}

				if actualRef.Key != expectedRef.Key {
					t.Errorf("SecretKeyRef[%q].Key = %q, expected %q", expectedEnvVar, actualRef.Key, expectedRef.Key)
				}

				// Verify the secret name matches the shared secret name used by the manager
				expectedSecretName := "test-secrets"
				if actualRef.Name != expectedSecretName {
					t.Errorf("Secret name %q doesn't match expected shared secret name %q", actualRef.Name, expectedSecretName)
				}
			}

			// Verify no unexpected refs
			if len(actualRefs) != len(tt.expectedRefs) {
				t.Errorf("Expected %d secret key refs, got %d", len(tt.expectedRefs), len(actualRefs))
			}
		})
	}
}

// isValidKubernetesSecretName checks basic Kubernetes DNS-1123 compliance for secret names
func isValidKubernetesSecretName(name string) bool {
	// Kubernetes secret names must be valid DNS-1123 subdomains:
	// - contain only lowercase alphanumeric characters or '-'
	// - start and end with alphanumeric character
	// - be no more than 253 characters

	if len(name) == 0 || len(name) > 253 {
		return false
	}

	if !isAlphanumeric(rune(name[0])) || !isAlphanumeric(rune(name[len(name)-1])) {
		return false
	}

	for _, char := range name {
		if !isAlphanumeric(char) && char != '-' {
			return false
		}
	}

	return true
}

func isAlphanumeric(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
}

func TestSecretNameValidation_EdgeCases(t *testing.T) {
	tests := []struct {
		secretName string
		isValid    bool
		reason     string
	}{
		{"valid-name", true, "simple valid name"},
		{"mcp-server-secrets", true, "our naming pattern"},
		{"a", true, "single character"},
		{"", false, "empty string"},
		{"Valid-Name", false, "contains uppercase"},
		{"name_with_underscore", false, "contains underscore"},
		{"name.with.dots", false, "contains dots"},
		{"name with spaces", false, "contains spaces"},
		{"-starts-with-dash", false, "starts with dash"},
		{"ends-with-dash-", false, "ends with dash"},
		{"123starts-with-number", true, "starts with number (allowed)"},
		{"ends-with-number123", true, "ends with number (allowed)"},
		{strings.Repeat("a", 253), true, "exactly 253 characters"},
		{strings.Repeat("a", 254), false, "254 characters (too long)"},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			isValid := isValidKubernetesSecretName(tt.secretName)
			if isValid != tt.isValid {
				t.Errorf("isValidKubernetesSecretName(%q) = %t, expected %t (%s)",
					tt.secretName, isValid, tt.isValid, tt.reason)
			}
		})
	}
}
