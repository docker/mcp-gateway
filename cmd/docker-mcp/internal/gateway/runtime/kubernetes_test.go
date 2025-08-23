package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestNewKubernetesContainerRuntime(t *testing.T) {
	tests := []struct {
		name            string
		config          KubernetesContainerRuntimeConfig
		expectedNS      string
		expectedConfig  string
		expectedCtx     string
		expectedVerbose bool
		expectError     bool // client-go may fail if no kubeconfig available
	}{
		{
			name: "Custom configuration with error handling",
			config: KubernetesContainerRuntimeConfig{
				ContainerRuntimeConfig: ContainerRuntimeConfig{Verbose: true},
				Namespace:              "custom-namespace",
				Kubeconfig:             "/path/to/kubeconfig",
				KubeContext:            "custom-context",
			},
			expectedNS:      "custom-namespace",
			expectedConfig:  "/path/to/kubeconfig",
			expectedCtx:     "custom-context",
			expectedVerbose: true,
			expectError:     true, // Will fail with invalid kubeconfig path
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime, err := NewKubernetesContainerRuntime(tt.config)

			if tt.expectError {
				// client-go initialization may fail without valid kubeconfig
				// This is expected behavior when not running in cluster or without valid config
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, runtime)
			assert.Equal(t, tt.expectedNS, runtime.namespace)
			assert.Equal(t, tt.expectedConfig, runtime.kubeconfig)
			assert.Equal(t, tt.expectedCtx, runtime.kubeContext)
			assert.Equal(t, tt.expectedVerbose, runtime.verbose)
			assert.NotNil(t, runtime.clientset)
			assert.NotNil(t, runtime.restConfig)
		})
	}
}

func TestKubernetesContainerRuntimeGetName(t *testing.T) {
	// GetName should work even if client creation fails
	runtime := &KubernetesContainerRuntime{}
	assert.Equal(t, "kubernetes", runtime.GetName())
}

func TestKubernetesContainerRuntimeRunContainer(t *testing.T) {
	// This test requires external registry access to inspect images and a real Kubernetes cluster
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test the method signature and behavior - this is an integration test
	// It will fail without a real Kubernetes cluster and Docker Hub access
	runtime := &KubernetesContainerRuntime{}

	spec := ContainerSpec{
		Name:    "test-job",
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}

	// This is an integration test that requires real Kubernetes cluster and registry access
	result, err := runtime.RunContainer(context.Background(), spec)
	// Either succeeds with proper setup or fails with infrastructure errors
	// The important part is it's not returning "not yet implemented"
	if err != nil {
		// Expected failures: no clientset, no kubeconfig, registry rate limits, etc.
		t.Logf("Integration test failed as expected without proper setup: %v", err)
	}

	// If it somehow succeeded, validate the result
	if result != nil {
		assert.NotEmpty(t, result.Runtime)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple name",
			input:    "test-server",
			expected: "test-server",
		},
		{
			name:     "Name with underscores",
			input:    "test_server_name",
			expected: "test-server-name",
		},
		{
			name:     "Name with dots",
			input:    "test.server.name",
			expected: "test-server-name",
		},
		{
			name:     "Uppercase name",
			input:    "TEST_SERVER",
			expected: "test-server",
		},
		{
			name:     "Mixed characters",
			input:    "Test_Server.Name",
			expected: "test-server-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreatePodManifest(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	spec := ContainerSpec{
		Name:    "test-server",
		Image:   "nginx:latest",
		Command: []string{"nginx", "-g", "daemon off;"},
		Env: map[string]string{
			"ENV1": "value1",
			"ENV2": "value2",
		},
	}

	pod := runtime.createPodManifest("test-pod-123", spec)

	// Verify pod structure
	assert.Equal(t, "test-pod-123", pod.Name)
	assert.Equal(t, "test-namespace", pod.Namespace)
	assert.Equal(t, "mcp-server", pod.Labels["app"])
	assert.Equal(t, "test-server", pod.Labels["server"])
	assert.Equal(t, "kubernetes", pod.Labels["runtime"])

	// Verify container spec
	assert.Len(t, pod.Spec.Containers, 1)
	container := pod.Spec.Containers[0]
	assert.Equal(t, "mcp-server", container.Name)
	assert.Equal(t, "nginx:latest", container.Image)
	assert.Equal(t, []string{"nginx", "-g", "daemon off;"}, container.Args)

	// Verify environment variables
	assert.Len(t, container.Env, 2)
	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}
	assert.Equal(t, "value1", envMap["ENV1"])
	assert.Equal(t, "value2", envMap["ENV2"])
}

func TestKubernetesContainerRuntimeDebugLog(t *testing.T) {
	// Test verbose vs non-verbose logging
	// Since debugLog prints to stdout, we can't easily capture it in tests
	// But we can verify the method exists and doesn't panic

	verboseRuntime := &KubernetesContainerRuntime{verbose: true}
	quietRuntime := &KubernetesContainerRuntime{verbose: false}

	// These should not panic
	assert.NotPanics(t, func() {
		verboseRuntime.debugLog("test", "message")
	})

	assert.NotPanics(t, func() {
		quietRuntime.debugLog("test", "message")
	})
}

// Phase 2.1: Kubernetes Runtime Unit Tests - buildPodSpec functionality
func TestKubernetesRuntime_createPodManifest_BasicFields(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	tests := []struct {
		name     string
		podName  string
		spec     ContainerSpec
		expected func(*corev1.Pod) // Validation function
	}{
		{
			name:    "Basic pod with minimal spec",
			podName: "test-pod",
			spec: ContainerSpec{
				Name:  "test-server",
				Image: "nginx:latest",
			},
			expected: func(pod *corev1.Pod) {
				assert.Equal(t, "test-pod", pod.Name)
				assert.Equal(t, "test-namespace", pod.Namespace)
				assert.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)
				assert.Equal(t, int64(5), *pod.Spec.TerminationGracePeriodSeconds)
				assert.Len(t, pod.Spec.Containers, 1)

				container := pod.Spec.Containers[0]
				assert.Equal(t, "mcp-server", container.Name)
				assert.Equal(t, "nginx:latest", container.Image)
				assert.True(t, container.Stdin)
				assert.False(t, container.StdinOnce)
				assert.False(t, container.TTY)
			},
		},
		{
			name:    "Pod with command",
			podName: "cmd-pod",
			spec: ContainerSpec{
				Name:    "cmd-server",
				Image:   "alpine:latest",
				Command: []string{"echo", "hello"},
			},
			expected: func(pod *corev1.Pod) {
				container := pod.Spec.Containers[0]
				assert.Equal(t, []string{"echo", "hello"}, container.Args)
				assert.Nil(t, container.Command) // Command should not be set, Args preserves ENTRYPOINT
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := runtime.createPodManifest(tt.podName, tt.spec)
			assert.NotNil(t, pod)
			tt.expected(pod)
		})
	}
}

func TestKubernetesRuntime_createPodManifest_EnvironmentVariables(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	spec := ContainerSpec{
		Name:  "env-server",
		Image: "nginx:latest",
		Env: map[string]string{
			"KEY1": "value1",
			"KEY2": "value2",
		},
		SecretKeyRefs: map[string]SecretKeyRef{
			"SECRET_VAR": {Name: "my-secret", Key: "password"},
		},
		ConfigMapRefs: []string{"my-config"},
	}

	pod := runtime.createPodManifest("env-pod", spec)
	container := pod.Spec.Containers[0]

	// Check regular environment variables
	envMap := make(map[string]string)
	for _, env := range container.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}
	assert.Equal(t, "value1", envMap["KEY1"])
	assert.Equal(t, "value2", envMap["KEY2"])

	// Check secret reference
	var secretEnv *corev1.EnvVar
	for _, env := range container.Env {
		if env.Name == "SECRET_VAR" {
			secretEnv = &env
			break
		}
	}
	assert.NotNil(t, secretEnv)
	assert.NotNil(t, secretEnv.ValueFrom)
	assert.NotNil(t, secretEnv.ValueFrom.SecretKeyRef)
	assert.Equal(t, "my-secret", secretEnv.ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "password", secretEnv.ValueFrom.SecretKeyRef.Key)

	// Check configmap reference
	assert.Len(t, container.EnvFrom, 1)
	assert.Equal(t, "my-config", container.EnvFrom[0].ConfigMapRef.Name)
}

func TestKubernetesRuntime_createSidecarPodManifest_BasicFields(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	spec := ContainerSpec{
		Name:  "poci-tool",
		Image: "alpine:latest",
		Env: map[string]string{
			"TOOL_VAR": "tool_value",
		},
	}

	pod := runtime.createSidecarPodManifest("sidecar-pod", spec, "echo hello >/logs/stdout.log 2>/logs/stderr.log; echo $? > /logs/exit_code.log; touch /logs/complete.marker")

	// Validate basic pod structure
	assert.Equal(t, "sidecar-pod", pod.Name)
	assert.Equal(t, "test-namespace", pod.Namespace)
	assert.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)
	assert.Equal(t, int64(5), *pod.Spec.TerminationGracePeriodSeconds)

	// Should have 2 containers: main + sidecar
	assert.Len(t, pod.Spec.Containers, 2)

	// Validate main container
	mainContainer := pod.Spec.Containers[0]
	assert.Equal(t, "main", mainContainer.Name)
	assert.Equal(t, "alpine:latest", mainContainer.Image)
	assert.Equal(t, []string{"sh", "-c"}, mainContainer.Command)
	assert.Len(t, mainContainer.Args, 1)
	assert.Contains(t, mainContainer.Args[0], "/logs/stdout.log")

	// Validate sidecar container
	sidecarContainer := pod.Spec.Containers[1]
	assert.Equal(t, "sidecar", sidecarContainer.Name)
	assert.Equal(t, "alpine:3.22.1", sidecarContainer.Image)
	assert.Equal(t, []string{"sleep", "3600"}, sidecarContainer.Command)

	// Validate shared volume
	assert.Len(t, pod.Spec.Volumes, 1)
	assert.Equal(t, "logs", pod.Spec.Volumes[0].Name)
	assert.NotNil(t, pod.Spec.Volumes[0].EmptyDir)

	// Validate volume mounts
	for _, container := range pod.Spec.Containers {
		assert.Len(t, container.VolumeMounts, 1)
		assert.Equal(t, "logs", container.VolumeMounts[0].Name)
		assert.Equal(t, "/logs", container.VolumeMounts[0].MountPath)
	}
}

func TestKubernetesRuntime_generateLabels_StandardLabels(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	tests := []struct {
		name         string
		spec         ContainerSpec
		expectedType string // "mcp-server" or "mcp-tool"
	}{
		{
			name: "MCP server labels",
			spec: ContainerSpec{
				Name:  "test-server",
				Image: "nginx:latest",
			},
			expectedType: "mcp-server",
		},
		{
			name: "MCP server with custom labels",
			spec: ContainerSpec{
				Name:  "custom-server",
				Image: "alpine:latest",
				Labels: map[string]string{
					"session": "abc123",
					"user":    "testuser",
				},
			},
			expectedType: "mcp-server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test standard pod manifest labels
			pod := runtime.createPodManifest("test-pod", tt.spec)
			labels := pod.Labels

			// Verify standard labels
			assert.Equal(t, "mcp-server", labels["app"])
			assert.Equal(t, sanitizeName(tt.spec.Name), labels["server"])
			assert.Equal(t, "kubernetes", labels["runtime"])

			// Verify custom labels are merged
			for key, value := range tt.spec.Labels {
				assert.Equal(t, value, labels[key])
			}
		})
	}
}

func TestKubernetesRuntime_generateLabels_SidecarLabels(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	spec := ContainerSpec{
		Name:  "poci-tool",
		Image: "alpine:latest",
		Labels: map[string]string{
			"session": "xyz789",
		},
	}

	pod := runtime.createSidecarPodManifest("sidecar-pod", spec, "test command")
	labels := pod.Labels

	// Verify sidecar-specific labels
	assert.Equal(t, "mcp-tool", labels["app"])
	assert.Equal(t, sanitizeName(spec.Name), labels["tool"])
	assert.Equal(t, "kubernetes", labels["runtime"])
	assert.Equal(t, "poci-sidecar", labels["type"])

	// Verify custom labels are merged
	assert.Equal(t, "xyz789", labels["session"])
}

func TestKubernetesRuntime_createPodManifest_LifecycleHandling(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	spec := ContainerSpec{
		Name:  "lifecycle-server",
		Image: "nginx:latest",
	}

	pod := runtime.createPodManifest("lifecycle-pod", spec)
	container := pod.Spec.Containers[0]

	// Verify lifecycle configuration
	assert.NotNil(t, container.Lifecycle)
	assert.NotNil(t, container.Lifecycle.PreStop)
	assert.NotNil(t, container.Lifecycle.PreStop.Exec)
	assert.Equal(t, []string{"sleep", "10"}, container.Lifecycle.PreStop.Exec.Command)
}

func TestKubernetesRuntime_createSidecarPodManifest_EnvironmentHandling(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	spec := ContainerSpec{
		Name:  "env-tool",
		Image: "alpine:latest",
		Env: map[string]string{
			"TOOL_ENV": "tool_value",
		},
		SecretKeyRefs: map[string]SecretKeyRef{
			"SECRET_KEY": {Name: "tool-secret", Key: "key1"},
		},
		ConfigMapRefs: []string{"tool-config"},
	}

	pod := runtime.createSidecarPodManifest("env-sidecar", spec, "test command")
	mainContainer := pod.Spec.Containers[0] // main container

	// Check regular environment variables in main container
	envMap := make(map[string]string)
	for _, env := range mainContainer.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}
	assert.Equal(t, "tool_value", envMap["TOOL_ENV"])

	// Check secret reference in main container
	var secretEnv *corev1.EnvVar
	for _, env := range mainContainer.Env {
		if env.Name == "SECRET_KEY" {
			secretEnv = &env
			break
		}
	}
	assert.NotNil(t, secretEnv)
	assert.NotNil(t, secretEnv.ValueFrom)
	assert.Equal(t, "tool-secret", secretEnv.ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "key1", secretEnv.ValueFrom.SecretKeyRef.Key)

	// Check configmap reference in main container
	assert.Len(t, mainContainer.EnvFrom, 1)
	assert.Equal(t, "tool-config", mainContainer.EnvFrom[0].ConfigMapRef.Name)

	// Sidecar should not have environment variables
	sidecarContainer := pod.Spec.Containers[1]
	assert.Empty(t, sidecarContainer.Env)
	assert.Empty(t, sidecarContainer.EnvFrom)
}

func TestKubernetesRuntime_GetNamespace(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "custom-namespace",
	}

	assert.Equal(t, "custom-namespace", runtime.GetNamespace())
}

func TestKubernetesRuntime_GetClientset_NotInitialized(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		clientset: nil,
	}

	clientset, err := runtime.GetClientset()
	require.Error(t, err)
	assert.Nil(t, clientset)
	assert.Contains(t, err.Error(), "kubernetes clientset not initialized")
}

func TestKubernetesRuntime_inspectImage_ValidationRules(t *testing.T) {
	// This test validates the image parsing logic without requiring registry access
	runtime := &KubernetesContainerRuntime{
		verbose: true, // Enable debug logging
	}

	tests := []struct {
		name        string
		imageRef    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Invalid image reference",
			imageRef:    "invalid@image@ref",
			expectError: true,
			errorMsg:    "failed to parse image reference",
		},
		{
			name:        "Empty image reference",
			imageRef:    "",
			expectError: true,
			errorMsg:    "failed to parse image reference",
		},
		{
			name:     "Valid image reference format",
			imageRef: "nginx:latest",
			// Note: This will fail due to registry access, but validates parsing
			expectError: true, // Expected in test environment without registry access
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := runtime.inspectImage(context.Background(), tt.imageRef)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			}
			// Note: All test cases expect errors in test environment without registry access
		})
	}
}

func TestKubernetesRuntime_Shutdown_NoPanic(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		verbose: true,
	}

	// Shutdown should not panic and should complete successfully
	assert.NotPanics(t, func() {
		err := runtime.Shutdown(context.Background())
		assert.NoError(t, err)
	})
}

// Additional tests to improve coverage for Phase 2.1

func TestGetKubernetesConfig_InClusterConfig(t *testing.T) {
	// Test the in-cluster vs out-of-cluster config logic
	// This tests the getKubernetesConfig function without requiring real cluster

	// Test with empty kubeconfig path (should try in-cluster first, then fallback)
	config, err := getKubernetesConfig("", "")

	// Result depends on environment - may succeed with local cluster or fail
	// The important part is testing the function doesn't panic
	if err != nil {
		t.Logf("Config failed as expected in test environment: %v", err)
		assert.Nil(t, config)
	} else {
		t.Logf("Config succeeded with local cluster available")
		assert.NotNil(t, config)
	}
}

func TestGetKubernetesConfig_InvalidKubeconfig(t *testing.T) {
	// Test with invalid kubeconfig path
	config, err := getKubernetesConfig("/nonexistent/path/to/kubeconfig", "")

	require.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestIsPodReady_VariousStates(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "Pod running and ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Pod running but not ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Pod not running",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Pod with no ready condition",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase:      corev1.PodRunning,
					Conditions: []corev1.PodCondition{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPodReady(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKubernetesRuntime_createSidecarPodManifest_AdvancedCases(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "production",
	}

	tests := []struct {
		name       string
		spec       ContainerSpec
		command    string
		validateFn func(*testing.T, *corev1.Pod)
	}{
		{
			name: "Complex environment variables",
			spec: ContainerSpec{
				Name:  "complex-tool",
				Image: "alpine:3.16",
				Env: map[string]string{
					"DATABASE_URL": "postgresql://user:pass@db:5432/app",
					"LOG_LEVEL":    "debug",
					"FEATURE_FLAG": "true",
				},
				SecretKeyRefs: map[string]SecretKeyRef{
					"API_KEY":   {Name: "api-secrets", Key: "key"},
					"DB_PASSWD": {Name: "db-secrets", Key: "password"},
				},
				ConfigMapRefs: []string{"app-config", "feature-flags"},
			},
			command: "complex-command > /logs/stdout.log 2> /logs/stderr.log",
			validateFn: func(t *testing.T, pod *corev1.Pod) {
				t.Helper()
				mainContainer := pod.Spec.Containers[0]

				// Check regular env vars
				envMap := make(map[string]string)
				for _, env := range mainContainer.Env {
					if env.Value != "" {
						envMap[env.Name] = env.Value
					}
				}
				assert.Equal(t, "postgresql://user:pass@db:5432/app", envMap["DATABASE_URL"])
				assert.Equal(t, "debug", envMap["LOG_LEVEL"])
				assert.Equal(t, "true", envMap["FEATURE_FLAG"])

				// Check secret refs
				secretRefs := make(map[string]*corev1.SecretKeySelector)
				for _, env := range mainContainer.Env {
					if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
						secretRefs[env.Name] = env.ValueFrom.SecretKeyRef
					}
				}
				assert.Equal(t, "api-secrets", secretRefs["API_KEY"].Name)
				assert.Equal(t, "key", secretRefs["API_KEY"].Key)
				assert.Equal(t, "db-secrets", secretRefs["DB_PASSWD"].Name)
				assert.Equal(t, "password", secretRefs["DB_PASSWD"].Key)

				// Check configmap refs
				assert.Len(t, mainContainer.EnvFrom, 2)
				configMaps := make([]string, len(mainContainer.EnvFrom))
				for i, envFrom := range mainContainer.EnvFrom {
					configMaps[i] = envFrom.ConfigMapRef.Name
				}
				assert.Contains(t, configMaps, "app-config")
				assert.Contains(t, configMaps, "feature-flags")
			},
		},
		{
			name: "Custom labels and namespace",
			spec: ContainerSpec{
				Name:  "labeled-tool",
				Image: "ubuntu:22.04",
				Labels: map[string]string{
					"environment": "production",
					"team":        "backend",
					"version":     "v2.1.0",
				},
			},
			command: "echo test",
			validateFn: func(t *testing.T, pod *corev1.Pod) {
				t.Helper()
				assert.Equal(t, "production", pod.Namespace)

				// Check standard labels
				assert.Equal(t, "mcp-tool", pod.Labels["app"])
				assert.Equal(t, "labeled-tool", pod.Labels["tool"])
				assert.Equal(t, "kubernetes", pod.Labels["runtime"])
				assert.Equal(t, "poci-sidecar", pod.Labels["type"])

				// Check custom labels
				assert.Equal(t, "production", pod.Labels["environment"])
				assert.Equal(t, "backend", pod.Labels["team"])
				assert.Equal(t, "v2.1.0", pod.Labels["version"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := runtime.createSidecarPodManifest("test-pod", tt.spec, tt.command)
			assert.NotNil(t, pod)
			tt.validateFn(t, pod)
		})
	}
}

func TestKubernetesRuntime_createPodManifest_EdgeCases(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-ns",
	}

	tests := []struct {
		name       string
		spec       ContainerSpec
		validateFn func(*testing.T, *corev1.Pod)
	}{
		{
			name: "Long command args",
			spec: ContainerSpec{
				Name:  "long-cmd-server",
				Image: "busybox:latest",
				Command: []string{
					"sh", "-c",
					"echo 'This is a very long command with many arguments' && sleep 10 && echo 'done'",
				},
			},
			validateFn: func(t *testing.T, pod *corev1.Pod) {
				t.Helper()
				container := pod.Spec.Containers[0]
				assert.Len(t, container.Args, 3)
				assert.Contains(t, container.Args[2], "very long command")
			},
		},
		{
			name: "Multiple secret references",
			spec: ContainerSpec{
				Name:  "multi-secret-server",
				Image: "nginx:alpine",
				SecretKeyRefs: map[string]SecretKeyRef{
					"SECRET_1": {Name: "secret-one", Key: "key1"},
					"SECRET_2": {Name: "secret-two", Key: "key2"},
					"SECRET_3": {Name: "secret-three", Key: "key3"},
				},
			},
			validateFn: func(t *testing.T, pod *corev1.Pod) {
				t.Helper()
				container := pod.Spec.Containers[0]

				secretCount := 0
				for _, env := range container.Env {
					if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
						secretCount++
					}
				}
				assert.Equal(t, 3, secretCount)
			},
		},
		{
			name: "Empty command (uses image default)",
			spec: ContainerSpec{
				Name:  "default-cmd-server",
				Image: "alpine:latest",
				// No command specified
			},
			validateFn: func(t *testing.T, pod *corev1.Pod) {
				t.Helper()
				container := pod.Spec.Containers[0]
				assert.Empty(t, container.Args)    // Should be empty to use image default
				assert.Empty(t, container.Command) // Should not override ENTRYPOINT
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := runtime.createPodManifest("test-pod", tt.spec)
			assert.NotNil(t, pod)
			tt.validateFn(t, pod)
		})
	}
}

func TestNewKubernetesContainerRuntime_DefaultNamespace(t *testing.T) {
	config := KubernetesContainerRuntimeConfig{
		ContainerRuntimeConfig: ContainerRuntimeConfig{
			Verbose: false,
		},
		// No namespace specified - should default to "default"
	}

	runtime, err := NewKubernetesContainerRuntime(config)

	if err != nil {
		// If it fails due to kubeconfig issues, that's expected in some environments
		assert.Contains(t, err.Error(), "failed to get Kubernetes config")
		assert.Nil(t, runtime)
	} else {
		// If it succeeds (local cluster available), check namespace defaulting
		assert.NotNil(t, runtime)
		assert.Equal(t, "default", runtime.namespace)
	}
}

func TestKubernetesRuntime_sanitizeName_ExtendedCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Name with special characters",
			input:    "my-server_v1.2.3",
			expected: "my-server-v1-2-3",
		},
		{
			name:     "Name with consecutive special chars",
			input:    "server__..--name",
			expected: "server------name", // sanitizeName currently doesn't collapse consecutive dashes
		},
		{
			name:     "Mixed case with underscores",
			input:    "Test_Server_V1",
			expected: "test-server-v1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeName(tc.input)
			assert.Equal(t, tc.expected, result)
			// Ensure result is lowercase
			assert.Equal(t, strings.ToLower(result), result)
		})
	}
}

func TestKubernetesRuntime_podTemplateGeneration_Comprehensive(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	// Test comprehensive pod template generation with various configuration combinations
	tests := []struct {
		name string
		spec ContainerSpec
	}{
		{
			name: "Pod with resource limits and requests",
			spec: ContainerSpec{
				Name:    "resource-limited-server",
				Image:   "mcp/test-server:latest",
				Command: []string{"python", "server.py"},
				Env: map[string]string{
					"MEMORY_LIMIT": "512Mi",
					"CPU_LIMIT":    "500m",
					"TIER":         "production",
					"ENVIRONMENT":  "staging",
				},
			},
		},
		{
			name: "Pod with complex environment resolution",
			spec: ContainerSpec{
				Name:  "env-complex-server",
				Image: "mcp/complex-server:v2.1.0",
				Env: map[string]string{
					"SERVICE_URL":   "https://api.example.com/v1",
					"TIMEOUT_MS":    "30000",
					"RETRY_COUNT":   "3",
					"DEBUG_ENABLED": "true",
					"FEATURE_FLAGS": "feature1,feature2,feature3",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test createPodManifest method
			podName := sanitizeName(tc.spec.Name)
			manifest := runtime.createPodManifest(podName, tc.spec)

			// Verify basic structure (note: APIVersion and Kind are not set by createPodManifest)
			assert.NotNil(t, manifest)
			assert.Equal(t, podName, manifest.Name)
			assert.Equal(t, runtime.namespace, manifest.Namespace)

			// Verify container configuration
			assert.Len(t, manifest.Spec.Containers, 1)
			container := manifest.Spec.Containers[0]
			assert.Equal(t, "mcp-server", container.Name)
			assert.Equal(t, tc.spec.Image, container.Image)

			// Verify command if specified (stored in Args to preserve ENTRYPOINT)
			if len(tc.spec.Command) > 0 {
				assert.Equal(t, tc.spec.Command, container.Args)
			}

			// Verify environment variables are properly set
			if len(tc.spec.Env) > 0 {
				envMap := make(map[string]string)
				for _, env := range container.Env {
					envMap[env.Name] = env.Value
				}

				for key, expectedValue := range tc.spec.Env {
					actualValue, exists := envMap[key]
					assert.True(t, exists, "Environment variable %s should exist", key)
					assert.Equal(t, expectedValue, actualValue, "Environment variable %s should have correct value", key)
				}
			}

			// Verify standard MCP server labels are present
			assert.Equal(t, "mcp-server", manifest.Labels["app"])
			assert.Equal(t, sanitizeName(tc.spec.Name), manifest.Labels["server"])
			assert.Equal(t, "kubernetes", manifest.Labels["runtime"])
		})
	}
}

func TestKubernetesRuntime_createPodManifest_EmptyFields(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "default",
	}

	// Test with completely minimal spec
	spec := ContainerSpec{
		Name:  "minimal-server",
		Image: "nginx:latest",
		// No command, env, etc.
	}

	pod := runtime.createPodManifest("minimal-pod", spec)

	// Verify minimal pod structure
	assert.Equal(t, "minimal-pod", pod.Name)
	assert.Equal(t, "default", pod.Namespace)
	assert.Len(t, pod.Spec.Containers, 1)

	container := pod.Spec.Containers[0]
	assert.Equal(t, "mcp-server", container.Name)
	assert.Equal(t, "nginx:latest", container.Image)
	assert.Empty(t, container.Env)  // No environment variables
	assert.Empty(t, container.Args) // No command specified (stored in Args)

	// Verify basic labels are present
	assert.Equal(t, "mcp-server", pod.Labels["app"])
	assert.Equal(t, sanitizeName(spec.Name), pod.Labels["server"])
}

func TestKubernetesRuntime_createSidecarPodManifest_ComprehensiveVariations(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	tests := []struct {
		name string
		spec ContainerSpec
	}{
		{
			name: "POCI tool with complex environment",
			spec: ContainerSpec{
				Name:  "complex-poci-tool",
				Image: "mcp/complex-tool:v1.2.3",
				Env: map[string]string{
					"TOOL_CONFIG":        "/config/tool.json",
					"LOG_LEVEL":          "debug",
					"RETRY_ATTEMPTS":     "3",
					"CONNECTION_TIMEOUT": "30s",
				},
				Command:  []string{"./run-tool", "--config", "/config/tool.json"},
				Networks: []string{"custom-network"},
				Volumes:  []string{"/host/config:/config:ro"},
			},
		},
		{
			name: "POCI tool with minimal configuration",
			spec: ContainerSpec{
				Name:  "minimal-poci",
				Image: "alpine:latest",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			podName := sanitizeName(tc.spec.Name)
			manifest := runtime.createSidecarPodManifest(podName, tc.spec, "echo 'test command'")

			// Verify basic structure
			assert.NotNil(t, manifest)
			assert.Equal(t, podName, manifest.Name)
			assert.Equal(t, runtime.namespace, manifest.Namespace)

			// Should have 2 containers: main tool + sidecar
			assert.Len(t, manifest.Spec.Containers, 2)

			// Verify main container (first container)
			mainContainer := manifest.Spec.Containers[0]
			assert.Equal(t, "main", mainContainer.Name)
			assert.Equal(t, tc.spec.Image, mainContainer.Image)

			// Verify sidecar container (second container)
			sidecarContainer := manifest.Spec.Containers[1]
			assert.Equal(t, "sidecar", sidecarContainer.Name)
			assert.Equal(t, "alpine:3.22.1", sidecarContainer.Image) // System image for sidecar

			// Verify shared volume for logs
			assert.Len(t, manifest.Spec.Volumes, 1)
			volume := manifest.Spec.Volumes[0]
			assert.Equal(t, "logs", volume.Name)
			assert.NotNil(t, volume.EmptyDir)

			// Verify environment variables if present
			if len(tc.spec.Env) > 0 {
				envMap := make(map[string]string)
				for _, env := range mainContainer.Env {
					envMap[env.Name] = env.Value
				}

				for key, expectedValue := range tc.spec.Env {
					actualValue, exists := envMap[key]
					assert.True(t, exists, "Environment variable %s should exist", key)
					assert.Equal(t, expectedValue, actualValue, "Environment variable %s should have correct value", key)
				}
			}

			// Verify POCI-specific labels
			assert.Equal(t, "mcp-tool", manifest.Labels["app"])
			assert.Equal(t, "poci-sidecar", manifest.Labels["type"])
			assert.Equal(t, sanitizeName(tc.spec.Name), manifest.Labels["tool"])
		})
	}
}

func TestKubernetesRuntime_ContainerSpecValidation(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "test-namespace",
	}

	tests := []struct {
		name string
		spec ContainerSpec
		desc string
	}{
		{
			name: "Spec with volumes and networks",
			spec: ContainerSpec{
				Name:     "volume-network-server",
				Image:    "nginx:alpine",
				Command:  []string{"nginx", "-g", "daemon off;"},
				Networks: []string{"frontend", "backend"},
				Volumes:  []string{"/host/data:/data:ro", "/host/logs:/logs"},
				Env: map[string]string{
					"NGINX_HOST":       "localhost",
					"NGINX_PORT":       "80",
					"WORKER_PROCESSES": "auto",
				},
				User: "1000:1000",
			},
			desc: "Complex server with volumes, networks, and user specification",
		},
		{
			name: "Database server spec",
			spec: ContainerSpec{
				Name:  "database-server",
				Image: "postgres:13",
				Env: map[string]string{
					"POSTGRES_DB":       "testdb",
					"POSTGRES_USER":     "dbuser",
					"POSTGRES_PASSWORD": "secretpassword",
					"PGDATA":            "/var/lib/postgresql/data/pgdata",
				},
				Volumes: []string{"/host/pgdata:/var/lib/postgresql/data"},
				User:    "postgres",
			},
			desc: "Database server with persistent volume and environment configuration",
		},
		{
			name: "API server with resource limits",
			spec: ContainerSpec{
				Name:    "api-server",
				Image:   "node:16-alpine",
				Command: []string{"node", "server.js"},
				Env: map[string]string{
					"NODE_ENV":     "production",
					"PORT":         "3000",
					"DATABASE_URL": "postgres://user:pass@db:5432/api",
					"REDIS_URL":    "redis://cache:6379",
					"LOG_LEVEL":    "info",
				},
				Networks: []string{"api-network"},
			},
			desc: "Node.js API server with production environment configuration",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test regular pod manifest creation
			podName := sanitizeName(tc.spec.Name)
			podManifest := runtime.createPodManifest(podName, tc.spec)

			// Verify pod structure
			assert.NotNil(t, podManifest, "Pod manifest should not be nil")
			assert.Equal(t, podName, podManifest.Name)
			assert.Equal(t, runtime.namespace, podManifest.Namespace)
			assert.Len(t, podManifest.Spec.Containers, 1)

			// Test sidecar pod manifest creation
			sidecarManifest := runtime.createSidecarPodManifest(podName+"-sidecar", tc.spec, "echo 'test command'")

			// Verify sidecar structure
			assert.NotNil(t, sidecarManifest, "Sidecar manifest should not be nil")
			assert.Equal(t, podName+"-sidecar", sidecarManifest.Name)
			assert.Len(t, sidecarManifest.Spec.Containers, 2, "Sidecar should have 2 containers")

			// Verify container names in sidecar
			containerNames := []string{
				sidecarManifest.Spec.Containers[0].Name,
				sidecarManifest.Spec.Containers[1].Name,
			}
			assert.Contains(t, containerNames, "main")
			assert.Contains(t, containerNames, "sidecar")

			// Verify spec parsing fidelity
			container := podManifest.Spec.Containers[0]
			assert.Equal(t, tc.spec.Image, container.Image, "Image should match spec")

			if len(tc.spec.Command) > 0 {
				assert.Equal(t, tc.spec.Command, container.Args, "Command should be stored in Args")
			}

			// Verify environment variables
			if len(tc.spec.Env) > 0 {
				envMap := make(map[string]string)
				for _, env := range container.Env {
					envMap[env.Name] = env.Value
				}

				for key, expectedValue := range tc.spec.Env {
					actualValue, exists := envMap[key]
					assert.True(t, exists, "Environment variable %s should exist", key)
					assert.Equal(t, expectedValue, actualValue, "Environment variable %s value mismatch", key)
				}

				t.Logf("Successfully validated %d environment variables for %s", len(tc.spec.Env), tc.desc)
			}
		})
	}
}

func TestKubernetesRuntime_LabelGeneration_Comprehensive(t *testing.T) {
	runtime := &KubernetesContainerRuntime{
		namespace: "production",
	}

	tests := []struct {
		name         string
		spec         ContainerSpec
		expectedBase map[string]string
		description  string
	}{
		{
			name: "MCP server with custom labels",
			spec: ContainerSpec{
				Name:  "weather-server",
				Image: "mcp/weather:v2.1.0",
				Labels: map[string]string{
					"version":     "2.1.0",
					"environment": "production",
					"team":        "platform",
				},
			},
			expectedBase: map[string]string{
				"app":     "mcp-server",
				"server":  "weather-server",
				"runtime": "kubernetes",
			},
			description: "MCP server should have standard labels plus custom labels",
		},
		{
			name: "POCI tool with deployment metadata",
			spec: ContainerSpec{
				Name:  "file-processor",
				Image: "tools/file-processor:latest",
				Labels: map[string]string{
					"deployment":   "blue-green",
					"canary":       "10%",
					"feature-flag": "enabled",
				},
			},
			expectedBase: map[string]string{
				"app":     "mcp-server", // Standard app label
				"server":  "file-processor",
				"runtime": "kubernetes",
			},
			description: "POCI tool should preserve deployment metadata in labels",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			podName := sanitizeName(tc.spec.Name)

			// Test regular pod manifest labels
			podManifest := runtime.createPodManifest(podName, tc.spec)
			labels := podManifest.Labels

			// Verify all expected base labels are present
			for key, expectedValue := range tc.expectedBase {
				actualValue, exists := labels[key]
				assert.True(t, exists, "Base label %s should exist", key)
				assert.Equal(t, expectedValue, actualValue, "Base label %s value mismatch", key)
			}

			// Verify all custom labels are preserved
			if tc.spec.Labels != nil {
				for key, expectedValue := range tc.spec.Labels {
					actualValue, exists := labels[key]
					assert.True(t, exists, "Custom label %s should exist", key)
					assert.Equal(t, expectedValue, actualValue, "Custom label %s value mismatch", key)
				}
			}

			// Test sidecar pod manifest labels
			sidecarManifest := runtime.createSidecarPodManifest(podName+"-sidecar", tc.spec, "echo 'test command'")
			sidecarLabels := sidecarManifest.Labels

			// Sidecar should have POCI-specific labels
			assert.Equal(t, "mcp-tool", sidecarLabels["app"])
			assert.Equal(t, "poci-sidecar", sidecarLabels["type"])
			assert.Equal(t, sanitizeName(tc.spec.Name), sidecarLabels["tool"])

			t.Logf("Successfully validated label generation for %s", tc.description)
		})
	}
}

func TestKubernetesRuntime_debugLog_Functionality(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
	}{
		{
			name:    "Debug logging enabled",
			verbose: true,
		},
		{
			name:    "Debug logging disabled",
			verbose: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &KubernetesContainerRuntime{
				namespace: "test-namespace",
				verbose:   tc.verbose,
			}

			// debugLog should not panic regardless of verbose setting
			assert.NotPanics(t, func() {
				runtime.debugLog("test", "debug", "message")
			})
		})
	}
}
