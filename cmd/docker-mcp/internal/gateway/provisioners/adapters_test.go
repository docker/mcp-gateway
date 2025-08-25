package provisioners

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

func TestAdaptServerConfigToSpec(t *testing.T) {
	tests := []struct {
		name         string
		serverConfig *catalog.ServerConfig
		provisioner  ProvisionerType
		expectError  bool
		validate     func(t *testing.T, spec ProvisionerSpec)
	}{
		{
			name: "Basic Docker server config",
			serverConfig: &catalog.ServerConfig{
				Name: "test-server",
				Spec: catalog.Server{
					Image:     "alpine:latest",
					Command:   []string{"echo", "hello"},
					LongLived: true,
				},
				Config:  map[string]any{},
				Secrets: map[string]string{},
			},
			provisioner: DockerProvisioner,
			expectError: false,
			validate: func(t *testing.T, spec ProvisionerSpec) {
				t.Helper()
				assert.Equal(t, "test-server", spec.Name)
				assert.Equal(t, "alpine:latest", spec.Image)
				assert.Equal(t, []string{"echo", "hello"}, spec.Command)
				assert.True(t, spec.LongLived)
				assert.False(t, spec.DisableNetwork)
			},
		},
		{
			name: "Server with environment variables",
			serverConfig: &catalog.ServerConfig{
				Name: "env-server",
				Spec: catalog.Server{
					Image: "test:latest",
					Env: []catalog.Env{
						{Name: "SIMPLE_VAR", Value: "simple_value"},
						{Name: "EMPTY_VAR", Value: ""},
					},
				},
				Config:  map[string]any{},
				Secrets: map[string]string{},
			},
			provisioner: DockerProvisioner,
			expectError: false,
			validate: func(t *testing.T, spec ProvisionerSpec) {
				t.Helper()
				assert.Equal(t, "simple_value", spec.Environment["SIMPLE_VAR"])
				assert.NotContains(t, spec.Environment, "EMPTY_VAR")
			},
		},
		{
			name: "Server with template environment variables",
			serverConfig: &catalog.ServerConfig{
				Name: "template-server",
				Spec: catalog.Server{
					Image: "test:latest",
					Env: []catalog.Env{
						{Name: "TEMPLATE_VAR", Value: "{{config.value}}"},
						{Name: "MIXED_VAR", Value: "prefix-{{config.suffix}}"},
					},
				},
				Config: map[string]any{
					"config": map[string]any{
						"value":  "template_result",
						"suffix": "end",
					},
				},
				Secrets: map[string]string{},
			},
			provisioner: DockerProvisioner,
			expectError: false,
			validate: func(t *testing.T, spec ProvisionerSpec) {
				t.Helper()
				// Templates are kept raw and resolved just-in-time by provisioner
				assert.Equal(t, "{{config.value}}", spec.Environment["TEMPLATE_VAR"])
				assert.Equal(t, "prefix-{{config.suffix}}", spec.Environment["MIXED_VAR"])
			},
		},
		{
			name: "Server with secrets",
			serverConfig: &catalog.ServerConfig{
				Name: "secret-server",
				Spec: catalog.Server{
					Image: "test:latest",
					Secrets: []catalog.Secret{
						{Name: "api_key", Env: "API_KEY"},
						{Name: "missing_secret", Env: "MISSING_SECRET"},
					},
				},
				Config: map[string]any{},
				Secrets: map[string]string{
					"api_key": "secret_value_123",
				},
			},
			provisioner: DockerProvisioner,
			expectError: false,
			validate: func(t *testing.T, spec ProvisionerSpec) {
				t.Helper()
				// Secrets are resolved just-in-time by provisioner, not in spec
				assert.NotNil(t, spec)
			},
		},
		{
			name: "Server with volumes",
			serverConfig: &catalog.ServerConfig{
				Name: "volume-server",
				Spec: catalog.Server{
					Image:   "test:latest",
					Volumes: []string{"/host:/container", "/empty:", ""},
				},
				Config:  map[string]any{},
				Secrets: map[string]string{},
			},
			provisioner: DockerProvisioner,
			expectError: false,
			validate: func(t *testing.T, spec ProvisionerSpec) {
				t.Helper()
				assert.Contains(t, spec.Volumes, "/host:/container")
				assert.Contains(t, spec.Volumes, "/empty:")
				assert.NotContains(t, spec.Volumes, "")
			},
		},
		{
			name: "Server with template volumes",
			serverConfig: &catalog.ServerConfig{
				Name: "template-volume-server",
				Spec: catalog.Server{
					Image:   "test:latest",
					Volumes: []string{"{{paths.source}}:/dest"},
				},
				Config: map[string]any{
					"paths": map[string]any{
						"source": "/resolved/source",
					},
				},
				Secrets: map[string]string{},
			},
			provisioner: DockerProvisioner,
			expectError: false,
			validate: func(t *testing.T, spec ProvisionerSpec) {
				t.Helper()
				assert.Contains(t, spec.Volumes, "/resolved/source:/dest")
			},
		},
		{
			name: "Network disabled server",
			serverConfig: &catalog.ServerConfig{
				Name: "isolated-server",
				Spec: catalog.Server{
					Image:          "test:latest",
					DisableNetwork: true,
				},
				Config:  map[string]any{},
				Secrets: map[string]string{},
			},
			provisioner: DockerProvisioner,
			expectError: false,
			validate: func(t *testing.T, spec ProvisionerSpec) {
				t.Helper()
				assert.True(t, spec.DisableNetwork)
			},
		},
		{
			name: "Kubernetes provisioner success",
			serverConfig: &catalog.ServerConfig{
				Name: "k8s-server",
				Spec: catalog.Server{
					Image: "test:latest",
				},
				Config:  map[string]any{},
				Secrets: map[string]string{},
			},
			provisioner: KubernetesProvisioner,
			expectError: false,
			validate: func(t *testing.T, spec ProvisionerSpec) {
				t.Helper()
				assert.Equal(t, "k8s-server", spec.Name)
				assert.Equal(t, "test:latest", spec.Image)
			},
		},
		{
			name: "Cloud provisioner not implemented",
			serverConfig: &catalog.ServerConfig{
				Name: "cloud-server",
				Spec: catalog.Server{
					Image: "test:latest",
				},
				Config:  map[string]any{},
				Secrets: map[string]string{},
			},
			provisioner: CloudProvisioner,
			expectError: true,
			validate:    nil,
		},
		{
			name:         "Nil server config",
			serverConfig: nil,
			provisioner:  DockerProvisioner,
			expectError:  true,
			validate:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := AdaptServerConfigToSpec(tt.serverConfig, tt.provisioner)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, spec)
			}
		})
	}
}

func TestAdaptForKubernetes(t *testing.T) {
	spec := ProvisionerSpec{
		Name:  "test-server",
		Image: "alpine",
	}

	result, err := adaptForKubernetes(spec, nil)
	require.NoError(t, err)
	assert.Equal(t, spec, result) // Should return the same spec
}

func TestResolveTemplatedEnvironmentVars(t *testing.T) {
	tests := []struct {
		name         string
		serverConfig *catalog.ServerConfig
		expected     map[string]string
	}{
		{
			name: "Simple environment variables",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Env: []catalog.Env{
						{Name: "VAR1", Value: "value1"},
						{Name: "VAR2", Value: "value2"},
						{Name: "EMPTY", Value: ""},
					},
				},
				Config: map[string]any{},
			},
			expected: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
		},
		{
			name: "Template environment variables (raw templates)",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Env: []catalog.Env{
						{Name: "TEMPLATE_VAR", Value: "{{config.key}}"},
						{Name: "MULTI_TEMPLATE", Value: "{{prefix}}-{{suffix}}"},
					},
				},
				Config: map[string]any{
					"config": map[string]any{
						"key": "resolved_value",
					},
					"prefix": "start",
					"suffix": "end",
				},
			},
			expected: map[string]string{
				"TEMPLATE_VAR":   "{{config.key}}",
				"MULTI_TEMPLATE": "{{prefix}}-{{suffix}}",
			},
		},
		{
			name: "Mixed template and simple variables",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Env: []catalog.Env{
						{Name: "SIMPLE", Value: "simple_value"},
						{Name: "TEMPLATE", Value: "{{dynamic.value}}"},
					},
				},
				Config: map[string]any{
					"dynamic": map[string]any{
						"value": "template_result",
					},
				},
			},
			expected: map[string]string{
				"SIMPLE":   "simple_value",
				"TEMPLATE": "{{dynamic.value}}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveTemplatedEnvironmentVars(tt.serverConfig)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveSecrets(t *testing.T) {
	tests := []struct {
		name         string
		serverConfig *catalog.ServerConfig
		expected     map[string]string
	}{
		{
			name: "Secrets with values",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Secrets: []catalog.Secret{
						{Name: "secret1", Env: "SECRET1"},
						{Name: "secret2", Env: "SECRET2"},
					},
				},
				Secrets: map[string]string{
					"secret1": "value1",
					"secret2": "value2",
				},
			},
			expected: map[string]string{
				"SECRET1": "value1",
				"SECRET2": "value2",
			},
		},
		{
			name: "Missing secrets",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Secrets: []catalog.Secret{
						{Name: "existing", Env: "EXISTING"},
						{Name: "missing", Env: "MISSING"},
					},
				},
				Secrets: map[string]string{
					"existing": "found_value",
				},
			},
			expected: map[string]string{
				"EXISTING": "found_value",
				"MISSING":  "<UNKNOWN>",
			},
		},
		{
			name: "No secrets",
			serverConfig: &catalog.ServerConfig{
				Spec:    catalog.Server{},
				Secrets: map[string]string{},
			},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveSecrets(tt.serverConfig)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveVolumes(t *testing.T) {
	tests := []struct {
		name         string
		serverConfig *catalog.ServerConfig
		expected     []string
	}{
		{
			name: "Simple volumes",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Volumes: []string{"/host:/container", "/data:/app/data:ro"},
				},
				Config: map[string]any{},
			},
			expected: []string{"/host:/container", "/data:/app/data:ro"},
		},
		{
			name: "Template volumes",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Volumes: []string{"{{volume.source}}:{{volume.target}}"},
				},
				Config: map[string]any{
					"volume": map[string]any{
						"source": "/resolved/source",
						"target": "/resolved/target",
					},
				},
			},
			expected: []string{"/resolved/source:/resolved/target"},
		},
		{
			name: "Volumes with empty entries",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Volumes: []string{"/valid:/volume", "", "/another:/volume"},
				},
				Config: map[string]any{},
			},
			expected: []string{"/valid:/volume", "/another:/volume"},
		},
		{
			name: "No volumes",
			serverConfig: &catalog.ServerConfig{
				Spec:   catalog.Server{},
				Config: map[string]any{},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveVolumes(tt.serverConfig)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractResourceLimits(t *testing.T) {
	serverConfig := &catalog.ServerConfig{
		Name: "test-server",
		Spec: catalog.Server{
			Image: "test:latest",
		},
	}

	limits := extractResourceLimits(serverConfig)

	// Currently returns empty limits as resources are handled at clientPool level
	assert.Equal(t, ResourceLimits{}, limits)
}
