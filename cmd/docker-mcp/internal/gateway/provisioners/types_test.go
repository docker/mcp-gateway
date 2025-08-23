package provisioners

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/runtime"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

// TestProvisionerInterface verifies the Provisioner interface definition
func TestProvisionerInterface(t *testing.T) {
	// This test ensures the interface compiles and can be used
	var provisioner Provisioner
	assert.Nil(t, provisioner) // Just verify the interface can be declared
}

// TestProvisionerSpec tests the ProvisionerSpec structure
func TestProvisionerSpec(t *testing.T) {
	spec := ProvisionerSpec{
		Name:        "test-server",
		Image:       "alpine:latest",
		Command:     []string{"echo", "test"},
		Environment: map[string]string{"ENV": "test"},
		Volumes:     []string{"/host:/container"},
		Ports: []PortMapping{
			{ContainerPort: 8080, Protocol: "tcp"},
		},
		Networks: []string{"test-network"},
		Resources: ResourceLimits{
			CPUs:   1.5,
			Memory: "512m",
		},
		DisableNetwork: false,
		LongLived:      true,
	}

	assert.Equal(t, "test-server", spec.Name)
	assert.Equal(t, "alpine:latest", spec.Image)
	assert.Equal(t, []string{"echo", "test"}, spec.Command)
	assert.Equal(t, "test", spec.Environment["ENV"])
	assert.Len(t, spec.Volumes, 1)
	assert.Len(t, spec.Ports, 1)
	assert.Equal(t, 8080, spec.Ports[0].ContainerPort)
	assert.Equal(t, "tcp", spec.Ports[0].Protocol)
	assert.Len(t, spec.Networks, 1)
	assert.InDelta(t, 1.5, spec.Resources.CPUs, 0.01)
	assert.Equal(t, "512m", spec.Resources.Memory)
	assert.False(t, spec.DisableNetwork)
	assert.True(t, spec.LongLived)
}

// TestPortMapping tests the PortMapping structure
func TestPortMapping(t *testing.T) {
	port := PortMapping{
		ContainerPort: 3000,
		Protocol:      "udp",
	}

	assert.Equal(t, 3000, port.ContainerPort)
	assert.Equal(t, "udp", port.Protocol)
}

// TestResourceLimits tests the ResourceLimits structure
func TestResourceLimits(t *testing.T) {
	limits := ResourceLimits{
		CPUs:   2.0,
		Memory: "1Gb",
	}

	assert.InDelta(t, 2.0, limits.CPUs, 0.01)
	assert.Equal(t, "1Gb", limits.Memory)
}

// mockProvisioner implements the Provisioner interface for testing
type mockProvisioner struct {
	name string
}

func (m *mockProvisioner) GetName() string {
	return m.name
}

func (m *mockProvisioner) PreValidateDeployment(_ context.Context, _ ProvisionerSpec) error {
	return nil
}

func (m *mockProvisioner) ProvisionServer(_ context.Context, _ ProvisionerSpec) (mcpclient.Client, func(), error) {
	return nil, func() {}, nil
}

func (m *mockProvisioner) Initialize(_ context.Context, _ ConfigResolver, _ map[string]*catalog.ServerConfig) error {
	return nil
}

func (m *mockProvisioner) Shutdown(_ context.Context) error {
	return nil
}

func (m *mockProvisioner) ApplyToolProviders(_ *runtime.ContainerSpec, _ string) {
	// Mock implementation - no-op
}

// TestMockProvisioner tests that our mock implements the interface correctly
func TestMockProvisioner(t *testing.T) {
	mock := &mockProvisioner{name: "mock"}

	// Verify it implements Provisioner interface
	var provisioner Provisioner = mock
	assert.Equal(t, "mock", provisioner.GetName())

	// Test PreValidateDeployment
	err := provisioner.PreValidateDeployment(context.Background(), ProvisionerSpec{})
	require.NoError(t, err)

	// Test ProvisionServer
	client, cleanup, err := provisioner.ProvisionServer(context.Background(), ProvisionerSpec{})
	assert.Nil(t, client)     // Mock returns nil
	assert.NotNil(t, cleanup) // Should return a function
	require.NoError(t, err)
}

func TestProvisionerType(t *testing.T) {
	tests := []struct {
		name        string
		pt          ProvisionerType
		expectedStr string
	}{
		{"Docker provisioner", DockerProvisioner, "docker"},
		{"Kubernetes provisioner", KubernetesProvisioner, "kubernetes"},
		{"Cloud provisioner", CloudProvisioner, "cloud"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedStr, tt.pt.String())
			assert.True(t, tt.pt.IsValid())
		})
	}
}

func TestParseProvisionerType(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    ProvisionerType
		expectError bool
	}{
		{"Parse docker", "docker", DockerProvisioner, false},
		{"Parse kubernetes", "kubernetes", KubernetesProvisioner, false},
		{"Parse k8s alias", "k8s", KubernetesProvisioner, false},
		{"Parse cloud", "cloud", CloudProvisioner, false},
		{"Parse invalid", "invalid", DockerProvisioner, true},
		{"Parse empty", "", DockerProvisioner, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseProvisionerType(tt.input)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestProvisionerTypeIsValid(t *testing.T) {
	// Valid types
	assert.True(t, DockerProvisioner.IsValid())
	assert.True(t, KubernetesProvisioner.IsValid())
	assert.True(t, CloudProvisioner.IsValid())

	// Invalid type (outside range)
	invalidType := ProvisionerType(999)
	assert.False(t, invalidType.IsValid())
	assert.Equal(t, "unknown", invalidType.String())
}
