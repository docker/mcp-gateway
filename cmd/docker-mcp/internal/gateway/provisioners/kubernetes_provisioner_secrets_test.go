package provisioners

import (
	"testing"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/runtime"
)

// mockKubernetesSecretManager implements KubernetesSecretManager for testing
type mockKubernetesSecretManager struct {
	secretSpecs   map[string]map[string]map[string]string // serverName -> secretName -> secretData
	secretKeyRefs map[string]map[string]SecretKeyRef      // serverName -> envVar -> SecretKeyRef
}

func (m *mockKubernetesSecretManager) GetSecretSpecs(serverName string) map[string]map[string]string {
	if specs, exists := m.secretSpecs[serverName]; exists {
		return specs
	}
	return map[string]map[string]string{}
}

func (m *mockKubernetesSecretManager) GetSecretKeyRefs(serverName string) map[string]SecretKeyRef {
	if refs, exists := m.secretKeyRefs[serverName]; exists {
		return refs
	}
	return map[string]SecretKeyRef{}
}

// mockConfigResolver for buildContainerSpec tests
type mockBuildConfigResolver struct {
	environment map[string]map[string]string // serverName -> {envVar -> value}
	commands    map[string][]string          // serverName -> command
}

func (m *mockBuildConfigResolver) ResolveSecrets(_ string) map[string]string {
	return map[string]string{} // Not used in buildContainerSpec
}

func (m *mockBuildConfigResolver) ResolveEnvironment(serverName string) map[string]string {
	if env, exists := m.environment[serverName]; exists {
		return env
	}
	return map[string]string{}
}

func (m *mockBuildConfigResolver) ResolveCommand(serverName string) []string {
	if cmd, exists := m.commands[serverName]; exists {
		return cmd
	}
	return []string{}
}

func TestKubernetesProvisioner_buildContainerSpec_Secrets(t *testing.T) {
	tests := []struct {
		name               string
		spec               ProvisionerSpec
		configResolver     ConfigResolver
		secretManager      KubernetesSecretManager
		expectedEnv        map[string]string
		expectedSecretRefs map[string]runtime.SecretKeyRef
		sessionID          string
	}{
		{
			name: "no secrets",
			spec: ProvisionerSpec{
				Name:    "test-server",
				Image:   "alpine:latest",
				Command: []string{"sh", "-c", "echo hello"},
				Environment: map[string]string{
					"PUBLIC_VAR": "public-value",
				},
			},
			configResolver: &mockBuildConfigResolver{
				environment: map[string]map[string]string{
					"test-server": {
						"RESOLVED_VAR": "resolved-value",
					},
				},
				commands: map[string][]string{
					"test-server": {"sh", "-c", "echo hello"},
				},
			},
			secretManager: &mockKubernetesSecretManager{},
			expectedEnv: map[string]string{
				"PUBLIC_VAR":   "public-value",
				"RESOLVED_VAR": "resolved-value",
			},
			expectedSecretRefs: map[string]runtime.SecretKeyRef{},
			sessionID:          "test-session",
		},
		{
			name: "with secrets",
			spec: ProvisionerSpec{
				Name:    "dockerhub-server",
				Image:   "dockerhub/server:latest",
				Command: []string{"start-server"},
				Environment: map[string]string{
					"PUBLIC_VAR": "public-value",
				},
			},
			configResolver: &mockBuildConfigResolver{
				environment: map[string]map[string]string{
					"dockerhub-server": {
						"RESOLVED_VAR": "resolved-value",
					},
				},
				commands: map[string][]string{
					"dockerhub-server": {"start-server"},
				},
			},
			secretManager: &mockKubernetesSecretManager{
				secretKeyRefs: map[string]map[string]SecretKeyRef{
					"dockerhub-server": {
						"DOCKERHUB_USERNAME": {
							Name: "mcp-dockerhub-server-secrets",
							Key:  "dockerhub.username",
						},
						"DOCKERHUB_TOKEN": {
							Name: "mcp-dockerhub-server-secrets",
							Key:  "dockerhub.token",
						},
					},
				},
			},
			expectedEnv: map[string]string{
				"PUBLIC_VAR":   "public-value",
				"RESOLVED_VAR": "resolved-value",
			},
			expectedSecretRefs: map[string]runtime.SecretKeyRef{
				"DOCKERHUB_USERNAME": {
					Name: "mcp-dockerhub-server-secrets",
					Key:  "dockerhub.username",
				},
				"DOCKERHUB_TOKEN": {
					Name: "mcp-dockerhub-server-secrets",
					Key:  "dockerhub.token",
				},
			},
			sessionID: "dockerhub-session",
		},
		{
			name: "mixed environment and secrets",
			spec: ProvisionerSpec{
				Name:    "mixed-server",
				Image:   "mixed:latest",
				Command: []string{"run"},
				Environment: map[string]string{
					"PUBLIC_VAR":  "public",
					"ANOTHER_VAR": "another",
				},
				Volumes: []string{"/tmp:/data"},
			},
			configResolver: &mockBuildConfigResolver{
				environment: map[string]map[string]string{
					"mixed-server": {
						"TEMPLATE_VAR": "template-resolved",
					},
				},
				commands: map[string][]string{
					"mixed-server": {"custom", "command", "resolved"},
				},
			},
			secretManager: &mockKubernetesSecretManager{
				secretKeyRefs: map[string]map[string]SecretKeyRef{
					"mixed-server": {
						"SECRET_API_KEY": {
							Name: "mcp-mixed-server-secrets",
							Key:  "api.key",
						},
					},
				},
			},
			expectedEnv: map[string]string{
				"PUBLIC_VAR":   "public",
				"ANOTHER_VAR":  "another",
				"TEMPLATE_VAR": "template-resolved",
			},
			expectedSecretRefs: map[string]runtime.SecretKeyRef{
				"SECRET_API_KEY": {
					Name: "mcp-mixed-server-secrets",
					Key:  "api.key",
				},
			},
			sessionID: "mixed-session",
		},
		{
			name: "no resolvers",
			spec: ProvisionerSpec{
				Name:  "simple-server",
				Image: "simple:latest",
				Environment: map[string]string{
					"SIMPLE_VAR": "simple-value",
				},
			},
			configResolver: nil, // No config resolver
			secretManager:  nil, // No secret manager
			expectedEnv: map[string]string{
				"SIMPLE_VAR": "simple-value",
			},
			expectedSecretRefs: map[string]runtime.SecretKeyRef{},
			sessionID:          "simple-session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create KubernetesProvisioner with mocks
			provisioner := &KubernetesProvisionerImpl{
				configResolver: tt.configResolver,
				secretManager:  tt.secretManager,
				namespace:      "test-namespace",
				sessionID:      tt.sessionID,
			}

			// Call buildContainerSpec
			containerSpec := provisioner.buildContainerSpec(tt.spec)

			// Verify basic fields
			if containerSpec.Name != tt.spec.Name {
				t.Errorf("Expected Name=%q, got %q", tt.spec.Name, containerSpec.Name)
			}
			if containerSpec.Image != tt.spec.Image {
				t.Errorf("Expected Image=%q, got %q", tt.spec.Image, containerSpec.Image)
			}

			// Verify command (either from spec or resolved)
			expectedCommand := tt.spec.Command
			if tt.configResolver != nil {
				if resolved := tt.configResolver.ResolveCommand(tt.spec.Name); len(resolved) > 0 {
					expectedCommand = resolved
				}
			}
			if len(containerSpec.Command) != len(expectedCommand) {
				t.Errorf("Expected Command length=%d, got %d", len(expectedCommand), len(containerSpec.Command))
			} else {
				for i, expected := range expectedCommand {
					if containerSpec.Command[i] != expected {
						t.Errorf("Command[%d] = %q, expected %q", i, containerSpec.Command[i], expected)
					}
				}
			}

			// Verify environment variables
			if len(containerSpec.Env) != len(tt.expectedEnv) {
				t.Errorf("Expected %d environment variables, got %d", len(tt.expectedEnv), len(containerSpec.Env))
			}
			for expectedEnvVar, expectedValue := range tt.expectedEnv {
				actualValue, exists := containerSpec.Env[expectedEnvVar]
				if !exists {
					t.Errorf("Expected environment variable %q not found", expectedEnvVar)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("Environment variable %q = %q, expected %q", expectedEnvVar, actualValue, expectedValue)
				}
			}

			// Verify secret key refs
			if len(containerSpec.SecretKeyRefs) != len(tt.expectedSecretRefs) {
				t.Errorf("Expected %d secret key refs, got %d", len(tt.expectedSecretRefs), len(containerSpec.SecretKeyRefs))
			}
			for expectedEnvVar, expectedRef := range tt.expectedSecretRefs {
				actualRef, exists := containerSpec.SecretKeyRefs[expectedEnvVar]
				if !exists {
					t.Errorf("Expected secret key ref for %q not found", expectedEnvVar)
					continue
				}
				if actualRef.Name != expectedRef.Name {
					t.Errorf("SecretKeyRef[%q].Name = %q, expected %q", expectedEnvVar, actualRef.Name, expectedRef.Name)
				}
				if actualRef.Key != expectedRef.Key {
					t.Errorf("SecretKeyRef[%q].Key = %q, expected %q", expectedEnvVar, actualRef.Key, expectedRef.Key)
				}
			}

			// Verify session labels are added
			expectedLabelValue := tt.sessionID
			if expectedLabelValue != "" {
				actualSessionLabel, exists := containerSpec.Labels["mcp-gateway.docker.com/session"]
				if !exists {
					t.Errorf("Expected session label not found")
				} else if actualSessionLabel != expectedLabelValue {
					t.Errorf("Session label = %q, expected %q", actualSessionLabel, expectedLabelValue)
				}

				actualInstanceLabel, exists := containerSpec.Labels["app.kubernetes.io/instance"]
				if !exists {
					t.Errorf("Expected instance label not found")
				} else if actualInstanceLabel != expectedLabelValue {
					t.Errorf("Instance label = %q, expected %q", actualInstanceLabel, expectedLabelValue)
				}
			}

			// Verify other standard labels
			expectedLabels := map[string]string{
				"app.kubernetes.io/managed-by": "mcp-gateway",
				"app.kubernetes.io/component":  "mcp-server",
				"app.kubernetes.io/name":       tt.spec.Name,
			}
			for expectedLabel, expectedValue := range expectedLabels {
				actualValue, exists := containerSpec.Labels[expectedLabel]
				if !exists {
					t.Errorf("Expected label %q not found", expectedLabel)
				} else if actualValue != expectedValue {
					t.Errorf("Label %q = %q, expected %q", expectedLabel, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestKubernetesProvisioner_buildContainerSpec_LongLivedFlag(t *testing.T) {
	provisioner := &KubernetesProvisionerImpl{
		namespace: "test-namespace",
		sessionID: "test-session",
	}

	// Test long-lived spec
	longLivedSpec := ProvisionerSpec{
		Name:      "long-lived-server",
		Image:     "server:latest",
		LongLived: true,
	}
	containerSpec := provisioner.buildContainerSpec(longLivedSpec)
	if containerSpec.Persistent != true {
		t.Errorf("Expected Persistent=true for long-lived spec, got %t", containerSpec.Persistent)
	}

	// Test ephemeral spec
	ephemeralSpec := ProvisionerSpec{
		Name:      "ephemeral-server",
		Image:     "server:latest",
		LongLived: false,
	}
	containerSpec = provisioner.buildContainerSpec(ephemeralSpec)
	if containerSpec.Persistent != false {
		t.Errorf("Expected Persistent=false for ephemeral spec, got %t", containerSpec.Persistent)
	}
}
