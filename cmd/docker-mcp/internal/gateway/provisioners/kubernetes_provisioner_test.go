package provisioners

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/runtime"
)

// Simple test implementations without external mock library

// mockReadWriteCloser provides a simple ReadWriteCloser for testing
type mockReadWriteCloser struct{}

func (m *mockReadWriteCloser) Read(_ []byte) (n int, err error)  { return 0, nil }
func (m *mockReadWriteCloser) Write(p []byte) (n int, err error) { return len(p), nil }
func (m *mockReadWriteCloser) Close() error                      { return nil }

// testKubernetesContainerRuntime implements ContainerRuntime for testing
type testKubernetesContainerRuntime struct {
	name                 string
	runContainerResult   *runtime.ContainerResult
	runContainerError    error
	startContainerResult *runtime.ContainerHandle
	startContainerError  error
	stopContainerError   error
	lastStartSpec        runtime.ContainerSpec
}

func (t *testKubernetesContainerRuntime) GetName() string {
	if t.name == "" {
		return "test-kubernetes"
	}
	return t.name
}

func (t *testKubernetesContainerRuntime) RunContainer(_ context.Context, _ runtime.ContainerSpec) (*runtime.ContainerResult, error) {
	return t.runContainerResult, t.runContainerError
}

func (t *testKubernetesContainerRuntime) StartContainer(_ context.Context, spec runtime.ContainerSpec) (*runtime.ContainerHandle, error) {
	t.lastStartSpec = spec
	return t.startContainerResult, t.startContainerError
}

func (t *testKubernetesContainerRuntime) StopContainer(_ context.Context, _ *runtime.ContainerHandle) error {
	return t.stopContainerError
}

func (t *testKubernetesContainerRuntime) Shutdown(_ context.Context) error {
	// Test implementation - no cleanup needed
	return nil
}

// testConfigResolver implements ConfigResolver for testing
type testKubernetesConfigResolver struct {
	secrets     map[string]map[string]string
	environment map[string]map[string]string
	commands    map[string][]string
}

func (t *testKubernetesConfigResolver) ResolveSecrets(serverName string) map[string]string {
	if t.secrets == nil {
		return map[string]string{}
	}
	result := t.secrets[serverName]
	if result == nil {
		return map[string]string{}
	}
	return result
}

func (t *testKubernetesConfigResolver) ResolveEnvironment(serverName string) map[string]string {
	if t.environment == nil {
		return map[string]string{}
	}
	result := t.environment[serverName]
	if result == nil {
		return map[string]string{}
	}
	return result
}

func (t *testKubernetesConfigResolver) ResolveCommand(serverName string) []string {
	if t.commands == nil {
		return nil
	}
	return t.commands[serverName]
}

func TestNewKubernetesProvisioner(t *testing.T) {
	testRuntime := &testKubernetesContainerRuntime{}
	testResolver := &testKubernetesConfigResolver{}

	tests := []struct {
		name            string
		config          KubernetesProvisionerConfig
		expectedNS      string
		expectedVerbose bool
	}{
		{
			name: "Default namespace",
			config: KubernetesProvisionerConfig{
				ContainerRuntime: testRuntime,
				ConfigResolver:   testResolver,
				Verbose:          false,
			},
			expectedNS:      "default",
			expectedVerbose: false,
		},
		{
			name: "Custom namespace",
			config: KubernetesProvisionerConfig{
				ContainerRuntime: testRuntime,
				ConfigResolver:   testResolver,
				Namespace:        "custom-namespace",
				Verbose:          true,
			},
			expectedNS:      "custom-namespace",
			expectedVerbose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provisioner := NewKubernetesProvisioner(tt.config)

			assert.NotNil(t, provisioner)
			assert.Equal(t, tt.expectedNS, provisioner.namespace)
			assert.Equal(t, tt.expectedVerbose, provisioner.verbose)
			assert.Equal(t, testRuntime, provisioner.containerRuntime)
			assert.Equal(t, testResolver, provisioner.configResolver)
		})
	}
}

func TestKubernetesProvisionerGetName(t *testing.T) {
	provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
		ContainerRuntime: &testKubernetesContainerRuntime{},
	})

	assert.Equal(t, "kubernetes", provisioner.GetName())
}

func TestKubernetesProvisionerSetConfigResolver(t *testing.T) {
	provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
		ContainerRuntime: &testKubernetesContainerRuntime{},
	})

	resolver := &testKubernetesConfigResolver{}
	provisioner.SetConfigResolver(resolver)

	assert.Equal(t, resolver, provisioner.configResolver)
}

func TestKubernetesProvisionerPreValidateDeployment(t *testing.T) {
	provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
		ContainerRuntime: &testKubernetesContainerRuntime{},
	})

	tests := []struct {
		name        string
		spec        ProvisionerSpec
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid spec",
			spec: ProvisionerSpec{
				Name:  "test-server",
				Image: "nginx:latest",
			},
			expectError: false,
		},
		{
			name: "Missing name",
			spec: ProvisionerSpec{
				Image: "nginx:latest",
			},
			expectError: true,
			errorMsg:    "server name is required",
		},
		{
			name: "Missing image",
			spec: ProvisionerSpec{
				Name: "test-server",
			},
			expectError: true,
			errorMsg:    "container image is required for Kubernetes deployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provisioner.PreValidateDeployment(context.Background(), tt.spec)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKubernetesProvisionerProvisionServer_ContainerizedFailure(t *testing.T) {
	testRuntime := &testKubernetesContainerRuntime{
		startContainerError: fmt.Errorf("test container runtime error"),
	}
	testResolver := &testKubernetesConfigResolver{
		secrets: map[string]map[string]string{
			"test-server": {"SECRET_KEY": "secret-value"},
		},
		environment: map[string]map[string]string{
			"test-server": {"RESOLVED_ENV": "resolved-value"},
		},
		commands: map[string][]string{
			"test-server": {},
		},
	}

	provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
		ContainerRuntime: testRuntime,
		ConfigResolver:   testResolver,
	})

	spec := ProvisionerSpec{
		Name:  "test-server",
		Image: "nginx:latest",
		Environment: map[string]string{
			"ENV_VAR": "value",
		},
	}

	ctx := context.Background()
	client, cleanup, err := provisioner.ProvisionServer(ctx, spec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start ephemeral Kubernetes Pod")
	assert.Nil(t, client)
	assert.NotNil(t, cleanup)

	// Verify the container spec was built correctly
	lastSpec := testRuntime.lastStartSpec
	assert.Equal(t, "test-server", lastSpec.Name)
	assert.Equal(t, "nginx:latest", lastSpec.Image)
	assert.False(t, lastSpec.Persistent) // False because spec.LongLived is false
	assert.True(t, lastSpec.AttachStdio)
	assert.True(t, lastSpec.KeepStdinOpen)
	assert.Equal(t, "value", lastSpec.Env["ENV_VAR"])
	assert.Equal(t, "resolved-value", lastSpec.Env["RESOLVED_ENV"])
	// SECRET_KEY should not appear in regular env since secrets are handled via secret manager
}

func TestKubernetesProvisionerBuildContainerSpec(t *testing.T) {
	testResolver := &testKubernetesConfigResolver{
		secrets: map[string]map[string]string{
			"test-server": {"SECRET_KEY": "secret-value"},
		},
		environment: map[string]map[string]string{
			"test-server": {"RESOLVED_ENV": "resolved-value"},
		},
		commands: map[string][]string{
			"test-server": {"resolved-nginx", "-g", "daemon off;"},
		},
	}

	provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
		ContainerRuntime: &testKubernetesContainerRuntime{},
		ConfigResolver:   testResolver,
		Namespace:        "test-namespace",
	})

	spec := ProvisionerSpec{
		Name:    "test-server",
		Image:   "nginx:latest",
		Command: []string{"nginx", "-g", "daemon off;"},
		Environment: map[string]string{
			"ENV_VAR": "value",
		},
		Volumes:        []string{"/host:/container"},
		DisableNetwork: false,
	}

	containerSpec := provisioner.buildContainerSpec(spec)

	// Verify container spec properties
	assert.Equal(t, "test-server", containerSpec.Name)
	assert.Equal(t, "nginx:latest", containerSpec.Image)
	assert.Equal(t, []string{"resolved-nginx", "-g", "daemon off;"}, containerSpec.Command)
	assert.Equal(t, []string{"/host:/container"}, containerSpec.Volumes)
	assert.False(t, containerSpec.DisableNetwork)

	// Verify environment variables (regular env vars only, secrets handled separately)
	assert.Equal(t, "value", containerSpec.Env["ENV_VAR"])
	assert.Equal(t, "resolved-value", containerSpec.Env["RESOLVED_ENV"])
	// SECRET_KEY should not appear in regular env since secrets are handled via secret manager

	// Verify Kubernetes-specific settings
	assert.Empty(t, containerSpec.Networks)   // Should be empty for Kubernetes
	assert.False(t, containerSpec.Persistent) // False because spec.LongLived is false
	assert.True(t, containerSpec.AttachStdio)
	assert.True(t, containerSpec.KeepStdinOpen)
	assert.Equal(t, "no", containerSpec.RestartPolicy)
	assert.False(t, containerSpec.RemoveAfterRun)
	assert.True(t, containerSpec.Interactive)
	assert.False(t, containerSpec.Init) // Different from Docker
	assert.False(t, containerSpec.Privileged)
}

func TestKubernetesProvisionerBuildContainerSpecWithoutResolver(t *testing.T) {
	provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
		ContainerRuntime: &testKubernetesContainerRuntime{},
		ConfigResolver:   nil, // No resolver
	})

	spec := ProvisionerSpec{
		Name:    "test-server",
		Image:   "nginx:latest",
		Command: []string{"nginx"},
		Environment: map[string]string{
			"ENV_VAR": "value",
		},
	}

	containerSpec := provisioner.buildContainerSpec(spec)

	// Verify basic properties
	assert.Equal(t, "test-server", containerSpec.Name)
	assert.Equal(t, "nginx:latest", containerSpec.Image)
	assert.Equal(t, []string{"nginx"}, containerSpec.Command) // No resolution

	// Verify environment (only original, no resolved values)
	assert.Equal(t, "value", containerSpec.Env["ENV_VAR"])
	assert.Len(t, containerSpec.Env, 1) // Only the original env var
}

func TestKubernetesProvisionerSpecToServerConfig(t *testing.T) {
	provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
		ContainerRuntime: &testKubernetesContainerRuntime{},
	})

	spec := ProvisionerSpec{
		Name:    "test-server",
		Image:   "nginx:latest",
		Command: []string{"nginx", "-g", "daemon off;"},
		Environment: map[string]string{
			"ENV_VAR1": "value1",
			"ENV_VAR2": "value2",
		},
		Volumes:        []string{"/host1:/container1", "/host2:/container2"},
		DisableNetwork: true,
		LongLived:      true,
	}

	serverConfig := provisioner.specToServerConfig(spec)

	// Verify basic properties
	assert.Equal(t, "test-server", serverConfig.Name)
	assert.Equal(t, "nginx:latest", serverConfig.Spec.Image)
	assert.Equal(t, []string{"nginx", "-g", "daemon off;"}, serverConfig.Spec.Command)
	assert.True(t, serverConfig.Spec.DisableNetwork)
	assert.True(t, serverConfig.Spec.LongLived)

	// Verify environment variables
	assert.Len(t, serverConfig.Spec.Env, 2)
	envMap := make(map[string]string)
	for _, env := range serverConfig.Spec.Env {
		envMap[env.Name] = env.Value
	}
	assert.Equal(t, "value1", envMap["ENV_VAR1"])
	assert.Equal(t, "value2", envMap["ENV_VAR2"])

	// Verify volumes
	assert.Equal(t, []string{"/host1:/container1", "/host2:/container2"}, serverConfig.Spec.Volumes)

	// Verify secrets are empty (resolved just-in-time)
	assert.Empty(t, serverConfig.Spec.Secrets)
	assert.Empty(t, serverConfig.Secrets)

	// Verify config is empty
	assert.Empty(t, serverConfig.Config)
}

// TestKubernetesProvisionerInterface verifies the provisioner implements the Provisioner interface
func TestKubernetesProvisionerInterface(t *testing.T) {
	// Create test runtime that returns a valid handle for successful testing
	testRuntime := &testKubernetesContainerRuntime{
		startContainerResult: &runtime.ContainerHandle{
			ID:     "test-pod-12345",
			Stdin:  &mockReadWriteCloser{},
			Stdout: &mockReadWriteCloser{},
		},
	}

	provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
		ContainerRuntime: testRuntime,
	})

	// Verify it implements Provisioner interface
	var p Provisioner = provisioner
	assert.Equal(t, "kubernetes", p.GetName())

	// Verify interface methods exist and can be called
	err := p.PreValidateDeployment(context.Background(), ProvisionerSpec{
		Name:  "test",
		Image: "nginx:latest",
	})
	assert.NoError(t, err)

	// Note: We don't test ProvisionServer here because it would require
	// real MCP protocol initialization which is more of an integration test.
	// The provisioner interface compliance is verified by the compilation and
	// PreValidateDeployment test above.
}

func TestKubernetesProvisioner_PreValidateDeployment_ValidationCases(t *testing.T) {
	provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
		ContainerRuntime: &testKubernetesContainerRuntime{},
	})

	tests := []struct {
		name        string
		spec        ProvisionerSpec
		expectError bool
		errorMsg    string
	}{
		{
			name: "Complete valid spec",
			spec: ProvisionerSpec{
				Name:    "complete-server",
				Image:   "nginx:1.21",
				Command: []string{"nginx", "-g", "daemon off;"},
				Environment: map[string]string{
					"NODE_ENV": "production",
					"PORT":     "8080",
				},
				Volumes:        []string{"/data:/app/data"},
				DisableNetwork: false,
				LongLived:      true,
			},
			expectError: false,
		},
		{
			name: "Minimal valid spec",
			spec: ProvisionerSpec{
				Name:  "minimal",
				Image: "alpine",
			},
			expectError: false,
		},
		{
			name: "Empty name validation",
			spec: ProvisionerSpec{
				Name:  "",
				Image: "nginx:latest",
			},
			expectError: true,
			errorMsg:    "server name is required",
		},
		{
			name: "Whitespace-only name validation",
			spec: ProvisionerSpec{
				Name:  "   ",
				Image: "nginx:latest",
			},
			expectError: false, // Current implementation doesn't trim whitespace
		},
		{
			name: "Empty image validation",
			spec: ProvisionerSpec{
				Name:  "test-server",
				Image: "",
			},
			expectError: true,
			errorMsg:    "container image is required for Kubernetes deployment",
		},
		{
			name: "Image with tag",
			spec: ProvisionerSpec{
				Name:  "tagged-server",
				Image: "redis:6.2-alpine",
			},
			expectError: false,
		},
		{
			name: "Image with registry",
			spec: ProvisionerSpec{
				Name:  "registry-server",
				Image: "docker.io/library/postgres:13",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provisioner.PreValidateDeployment(context.Background(), tt.spec)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKubernetesProvisioner_PreValidateDeployment_ErrorScenarios(t *testing.T) {
	provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
		ContainerRuntime: &testKubernetesContainerRuntime{},
	})

	tests := []struct {
		name     string
		spec     ProvisionerSpec
		errorMsg string
	}{
		{
			name:     "Both name and image missing",
			spec:     ProvisionerSpec{},
			errorMsg: "server name is required", // First validation error
		},
		{
			name: "Special characters in name",
			spec: ProvisionerSpec{
				Name:  "test/server@domain.com",
				Image: "nginx",
			},
			errorMsg: "", // Should pass - validation doesn't restrict special chars
		},
		{
			name: "Very long name",
			spec: ProvisionerSpec{
				Name:  "very-long-server-name-that-might-exceed-kubernetes-naming-limits-and-cause-issues-with-resource-creation",
				Image: "nginx",
			},
			errorMsg: "", // Should pass - validation doesn't check name length
		},
		{
			name: "Image with digest",
			spec: ProvisionerSpec{
				Name:  "digest-server",
				Image: "nginx@sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4",
			},
			errorMsg: "", // Should pass
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provisioner.PreValidateDeployment(context.Background(), tt.spec)

			if tt.errorMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKubernetesProvisioner_buildContainerSpec_AllFieldVariations(t *testing.T) {
	tests := []struct {
		name               string
		configResolver     ConfigResolver
		sessionID          string
		spec               ProvisionerSpec
		expectedName       string
		expectedImage      string
		expectedCommand    []string
		expectedEnv        map[string]string
		expectedVolumes    []string
		expectedLabels     map[string]string
		expectedPersistent bool
		expectedAttach     bool
		expectedKeepStdin  bool
		expectedNetwork    bool
	}{
		{
			name:           "Basic spec without resolver",
			configResolver: nil,
			sessionID:      "",
			spec: ProvisionerSpec{
				Name:    "basic-server",
				Image:   "alpine:latest",
				Command: []string{"sh", "-c", "sleep 300"},
				Environment: map[string]string{
					"ENV1": "value1",
					"ENV2": "value2",
				},
				Volumes:        []string{"/tmp:/app/tmp"},
				DisableNetwork: false,
				LongLived:      false,
			},
			expectedName:       "basic-server",
			expectedImage:      "alpine:latest",
			expectedCommand:    []string{"sh", "-c", "sleep 300"},
			expectedEnv:        map[string]string{"ENV1": "value1", "ENV2": "value2"},
			expectedVolumes:    []string{"/tmp:/app/tmp"},
			expectedPersistent: false,
			expectedAttach:     true,
			expectedKeepStdin:  true,
			expectedNetwork:    false,
			expectedLabels: map[string]string{
				"app.kubernetes.io/managed-by": "mcp-gateway",
				"app.kubernetes.io/component":  "mcp-server",
				"app.kubernetes.io/name":       "basic-server",
			},
		},
		{
			name: "Spec with config resolver",
			configResolver: &testKubernetesConfigResolver{
				environment: map[string]map[string]string{
					"resolver-server": {
						"RESOLVED_ENV": "resolved-value",
						"CONFIG_VAR":   "config-value",
					},
				},
				commands: map[string][]string{
					"resolver-server": {"resolved-command", "--flag", "value"},
				},
			},
			sessionID: "test-session-123",
			spec: ProvisionerSpec{
				Name:    "resolver-server",
				Image:   "nginx:1.21",
				Command: []string{"nginx"}, // Will be replaced by resolver
				Environment: map[string]string{
					"ORIG_ENV": "original",
				},
				LongLived: true,
			},
			expectedName:       "resolver-server",
			expectedImage:      "nginx:1.21",
			expectedCommand:    []string{"resolved-command", "--flag", "value"},                                                           // Command still gets resolved
			expectedEnv:        map[string]string{"ORIG_ENV": "original", "RESOLVED_ENV": "resolved-value", "CONFIG_VAR": "config-value"}, // Env should resolve regardless of provider
			expectedVolumes:    nil,                                                                                                       // nil instead of empty slice
			expectedPersistent: true,
			expectedAttach:     true,
			expectedKeepStdin:  true,
			expectedNetwork:    false,
			expectedLabels: map[string]string{
				"app.kubernetes.io/managed-by":   "mcp-gateway",
				"app.kubernetes.io/component":    "mcp-server",
				"app.kubernetes.io/name":         "resolver-server",
				"app.kubernetes.io/instance":     "test-session-123",
				"mcp-gateway.docker.com/session": "test-session-123",
			},
		},
		{
			name:           "Network disabled spec",
			configResolver: nil,
			sessionID:      "network-test",
			spec: ProvisionerSpec{
				Name:           "network-disabled",
				Image:          "isolated:latest",
				DisableNetwork: true,
				LongLived:      false,
			},
			expectedName:       "network-disabled",
			expectedImage:      "isolated:latest",
			expectedCommand:    nil,
			expectedEnv:        map[string]string{},
			expectedVolumes:    nil, // nil instead of empty slice
			expectedPersistent: false,
			expectedAttach:     true,
			expectedKeepStdin:  true,
			expectedNetwork:    true,
			expectedLabels: map[string]string{
				"app.kubernetes.io/managed-by":   "mcp-gateway",
				"app.kubernetes.io/component":    "mcp-server",
				"app.kubernetes.io/name":         "network-disabled",
				"app.kubernetes.io/instance":     "network-test",
				"mcp-gateway.docker.com/session": "network-test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
				ContainerRuntime: &testKubernetesContainerRuntime{},
				ConfigResolver:   tt.configResolver,
				SessionID:        tt.sessionID,
				ConfigProvider:   DockerEngineConfigProvider, // Default behavior for environment resolution
			})

			containerSpec := provisioner.buildContainerSpec(tt.spec)

			// Verify basic properties
			assert.Equal(t, tt.expectedName, containerSpec.Name)
			assert.Equal(t, tt.expectedImage, containerSpec.Image)
			assert.Equal(t, tt.expectedCommand, containerSpec.Command)
			assert.Equal(t, tt.expectedVolumes, containerSpec.Volumes)

			// Verify environment variables
			assert.Equal(t, tt.expectedEnv, containerSpec.Env)

			// Verify labels
			for expectedKey, expectedValue := range tt.expectedLabels {
				actualValue, exists := containerSpec.Labels[expectedKey]
				assert.True(t, exists, "Expected label %s to exist", expectedKey)
				assert.Equal(t, expectedValue, actualValue, "Label %s value mismatch", expectedKey)
			}

			// Verify container behavior
			assert.Equal(t, tt.expectedPersistent, containerSpec.Persistent)
			assert.Equal(t, tt.expectedAttach, containerSpec.AttachStdio)
			assert.Equal(t, tt.expectedKeepStdin, containerSpec.KeepStdinOpen)
			assert.Equal(t, tt.expectedNetwork, containerSpec.DisableNetwork)

			// Verify Kubernetes-specific defaults
			assert.Empty(t, containerSpec.Networks, "Networks should be empty for Kubernetes")
			assert.Equal(t, "no", containerSpec.RestartPolicy)
			assert.False(t, containerSpec.RemoveAfterRun)
			assert.True(t, containerSpec.Interactive)
			assert.False(t, containerSpec.Init)
			assert.False(t, containerSpec.Privileged)
		})
	}
}

func TestKubernetesProvisioner_buildContainerSpec_Labels(t *testing.T) {
	tests := []struct {
		name               string
		sessionID          string
		serverName         string
		expectedLabels     map[string]string
		expectedLabelCount int
	}{
		{
			name:       "No session ID",
			sessionID:  "",
			serverName: "no-session-server",
			expectedLabels: map[string]string{
				"app.kubernetes.io/managed-by": "mcp-gateway",
				"app.kubernetes.io/component":  "mcp-server",
				"app.kubernetes.io/name":       "no-session-server",
			},
			expectedLabelCount: 3,
		},
		{
			name:       "With session ID",
			sessionID:  "session-abc123",
			serverName: "session-server",
			expectedLabels: map[string]string{
				"app.kubernetes.io/managed-by":   "mcp-gateway",
				"app.kubernetes.io/component":    "mcp-server",
				"app.kubernetes.io/name":         "session-server",
				"app.kubernetes.io/instance":     "session-abc123",
				"mcp-gateway.docker.com/session": "session-abc123",
			},
			expectedLabelCount: 5,
		},
		{
			name:       "Empty session ID (edge case)",
			sessionID:  "",
			serverName: "empty-session",
			expectedLabels: map[string]string{
				"app.kubernetes.io/managed-by": "mcp-gateway",
				"app.kubernetes.io/component":  "mcp-server",
				"app.kubernetes.io/name":       "empty-session",
			},
			expectedLabelCount: 3,
		},
		{
			name:       "Special characters in names",
			sessionID:  "session-with-dashes-123",
			serverName: "server-with-dots.and-dashes",
			expectedLabels: map[string]string{
				"app.kubernetes.io/managed-by":   "mcp-gateway",
				"app.kubernetes.io/component":    "mcp-server",
				"app.kubernetes.io/name":         "server-with-dots.and-dashes",
				"app.kubernetes.io/instance":     "session-with-dashes-123",
				"mcp-gateway.docker.com/session": "session-with-dashes-123",
			},
			expectedLabelCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
				ContainerRuntime: &testKubernetesContainerRuntime{},
				SessionID:        tt.sessionID,
			})

			spec := ProvisionerSpec{
				Name:  tt.serverName,
				Image: "test:latest",
			}

			containerSpec := provisioner.buildContainerSpec(spec)

			// Verify label count
			assert.Len(t, containerSpec.Labels, tt.expectedLabelCount)

			// Verify each expected label
			for expectedKey, expectedValue := range tt.expectedLabels {
				actualValue, exists := containerSpec.Labels[expectedKey]
				assert.True(t, exists, "Expected label %s to exist", expectedKey)
				assert.Equal(t, expectedValue, actualValue, "Label %s value mismatch", expectedKey)
			}
		})
	}
}

func TestKubernetesProvisioner_ErrorHandling(t *testing.T) {
	tests := []struct {
		name             string
		configResolver   ConfigResolver
		expectPanic      bool
		expectedEnvCount int
		expectedCmdLen   int
	}{
		{
			name:             "Nil config resolver",
			configResolver:   nil,
			expectPanic:      false,
			expectedEnvCount: 1, // Only original env
			expectedCmdLen:   2,
		},
		{
			name: "Config resolver with nil maps",
			configResolver: &testKubernetesConfigResolver{
				environment: nil, // Will return empty map
				commands:    nil, // Will return nil slice
			},
			expectPanic:      false,
			expectedEnvCount: 1, // Only original env
			expectedCmdLen:   0, // nil command because resolver returned nil
		},
		{
			name: "Config resolver with empty maps",
			configResolver: &testKubernetesConfigResolver{
				environment: map[string]map[string]string{},
				commands:    map[string][]string{},
			},
			expectPanic:      false,
			expectedEnvCount: 1, // Only original env
			expectedCmdLen:   0, // nil command because resolver returns nil for missing server
		},
		{
			name: "Config resolver with missing server",
			configResolver: &testKubernetesConfigResolver{
				environment: map[string]map[string]string{
					"other-server": {"OTHER_VAR": "other-value"},
				},
				commands: map[string][]string{
					"other-server": {"other-command"},
				},
			},
			expectPanic:      false,
			expectedEnvCount: 1, // Only original env
			expectedCmdLen:   0, // nil command because test-server not in resolver map
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provisioner := NewKubernetesProvisioner(KubernetesProvisionerConfig{
				ContainerRuntime: &testKubernetesContainerRuntime{},
				ConfigResolver:   tt.configResolver,
				ConfigProvider:   DockerEngineConfigProvider, // Default behavior
			})

			spec := ProvisionerSpec{
				Name:    "test-server",
				Image:   "nginx:latest",
				Command: []string{"nginx", "-g"},
				Environment: map[string]string{
					"ORIGINAL": "value",
				},
			}

			if tt.expectPanic {
				assert.Panics(t, func() {
					provisioner.buildContainerSpec(spec)
				})
			} else {
				containerSpec := provisioner.buildContainerSpec(spec)
				assert.Len(t, containerSpec.Env, tt.expectedEnvCount)
				assert.Len(t, containerSpec.Command, tt.expectedCmdLen)
			}
		})
	}
}
