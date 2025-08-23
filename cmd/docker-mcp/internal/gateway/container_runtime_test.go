package gateway

import (
	"context"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/provisioners"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/runtime"
)

// mockReadWriteCloser provides a simple ReadWriteCloser for testing
type mockReadWriteCloser struct {
	written []byte
	closed  bool
}

func (m *mockReadWriteCloser) Read(_ []byte) (n int, err error) { return 0, nil }
func (m *mockReadWriteCloser) Write(p []byte) (n int, err error) {
	m.written = append(m.written, p...)
	return len(p), nil
}

func (m *mockReadWriteCloser) Close() error {
	m.closed = true
	return nil
}

// mockContainerRuntime is a test implementation of ContainerRuntime
type mockContainerRuntime struct {
	capturedSpec          *runtime.ContainerSpec
	capturedStartSpec     *runtime.ContainerSpec
	capturedStopHandle    *runtime.ContainerHandle
	resultToReturn        *runtime.ContainerResult
	startHandleToReturn   *runtime.ContainerHandle
	errorToReturn         error
	startErrorToReturn    error
	stopErrorToReturn     error
	shutdownErrorToReturn error
	startContainerCalled  int
	stopContainerCalled   int
	runContainerCalled    int
	shutdownCalled        int
}

func (m *mockContainerRuntime) RunContainer(_ context.Context, spec runtime.ContainerSpec) (*runtime.ContainerResult, error) {
	m.runContainerCalled++
	m.capturedSpec = &spec
	if m.errorToReturn != nil {
		return nil, m.errorToReturn
	}
	if m.resultToReturn == nil {
		return &runtime.ContainerResult{
			Stdout:   "test output",
			Stderr:   "",
			ExitCode: 0,
			Success:  true,
			Runtime:  "mock",
		}, nil
	}
	return m.resultToReturn, nil
}

func (m *mockContainerRuntime) StartContainer(_ context.Context, spec runtime.ContainerSpec) (*runtime.ContainerHandle, error) {
	m.startContainerCalled++
	m.capturedStartSpec = &spec
	if m.startErrorToReturn != nil {
		return nil, m.startErrorToReturn
	}
	if m.startHandleToReturn == nil {
		// Return a default handle
		return &runtime.ContainerHandle{
			ID:     fmt.Sprintf("mock-container-%d", m.startContainerCalled),
			Stdin:  &mockReadWriteCloser{},
			Stdout: &mockReadWriteCloser{},
		}, nil
	}
	return m.startHandleToReturn, nil
}

func (m *mockContainerRuntime) StopContainer(_ context.Context, handle *runtime.ContainerHandle) error {
	m.stopContainerCalled++
	m.capturedStopHandle = handle
	if m.stopErrorToReturn != nil {
		return m.stopErrorToReturn
	}
	return nil
}

func (m *mockContainerRuntime) GetName() string {
	return "mock"
}

func (m *mockContainerRuntime) Shutdown(_ context.Context) error {
	m.shutdownCalled++
	if m.shutdownErrorToReturn != nil {
		return m.shutdownErrorToReturn
	}
	return nil
}

func TestRunToolContainer_UsesContainerRuntime(t *testing.T) {
	// Create a mock container runtime
	mockRuntime := &mockContainerRuntime{}

	// Create a mock docker provisioner
	dockerProvisioner := provisioners.NewDockerProvisioner(provisioners.DockerProvisionerConfig{
		ContainerRuntime: mockRuntime,
		Networks:         []string{"test-network"},
		Verbose:          false,
		Static:           false,
		BlockNetwork:     false,
		Cpus:             2,
		Memory:           "1g",
		LongLived:        false,
	})

	// Create provisioner map
	provisionerMap := make(map[provisioners.ProvisionerType]provisioners.Provisioner)
	provisionerMap[provisioners.DockerProvisioner] = dockerProvisioner

	// Create a client pool with the mock runtime and provisioner
	cp := newClientPool(ClientPoolConfig{
		Options: Options{
			Cpus:   2,
			Memory: "1g",
		},
		ContainerRuntime:   mockRuntime,
		Provisioners:       provisionerMap,
		DefaultProvisioner: provisioners.DockerProvisioner,
	})

	// Set networks on the client pool
	cp.SetNetworks([]string{"test-network"})

	// Create a test tool definition
	tool := catalog.Tool{
		Name: "test-tool",
		Container: catalog.Container{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
			Volumes: []string{"/tmp:/tmp"},
		},
	}

	// Create test parameters
	params := &mcp.CallToolParams{
		Name:      "test-tool",
		Arguments: map[string]any{"test": "value"},
	}

	// Execute the tool
	result, err := cp.runToolContainer(context.Background(), tool, params)
	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify result format
	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.IsError {
		t.Error("Expected successful result, got error result")
	}

	if len(result.Content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(result.Content))
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	if textContent.Text != "test output" {
		t.Errorf("Expected 'test output', got '%s'", textContent.Text)
	}

	// Verify that the container runtime was called with correct spec
	if mockRuntime.capturedSpec == nil {
		t.Fatal("Expected container runtime to be called, but capturedSpec is nil")
	}

	spec := mockRuntime.capturedSpec

	// Verify key fields were set correctly
	if spec.Name != "test-tool" {
		t.Errorf("Expected spec.Name='test-tool', got '%s'", spec.Name)
	}

	if spec.Image != "alpine:latest" {
		t.Errorf("Expected spec.Image='alpine:latest', got '%s'", spec.Image)
	}

	if len(spec.Command) != 2 || spec.Command[0] != "echo" || spec.Command[1] != "hello" {
		t.Errorf("Expected spec.Command=['echo', 'hello'], got %v", spec.Command)
	}

	if len(spec.Networks) != 1 || spec.Networks[0] != "test-network" {
		t.Errorf("Expected spec.Networks=['test-network'], got %v", spec.Networks)
	}

	if len(spec.Volumes) != 1 || spec.Volumes[0] != "/tmp:/tmp" {
		t.Errorf("Expected spec.Volumes=['/tmp:/tmp'], got %v", spec.Volumes)
	}

	if spec.CPUs != 2 {
		t.Errorf("Expected spec.CPUs=2, got %d", spec.CPUs)
	}

	if spec.Memory != "1g" {
		t.Errorf("Expected spec.Memory='1g', got '%s'", spec.Memory)
	}

	// Verify container behavior flags
	if !spec.RemoveAfterRun {
		t.Error("Expected spec.RemoveAfterRun=true")
	}

	if !spec.Interactive {
		t.Error("Expected spec.Interactive=true")
	}

	if !spec.Init {
		t.Error("Expected spec.Init=true")
	}

	if spec.Privileged {
		t.Error("Expected spec.Privileged=false")
	}

	if spec.DisableNetwork {
		t.Error("Expected spec.DisableNetwork=false")
	}
}

func TestRunToolContainer_HandlesContainerRuntimeError(t *testing.T) {
	// Create a mock container runtime that returns an error
	mockRuntime := &mockContainerRuntime{
		errorToReturn: &mockError{message: "Container runtime error"},
	}

	// Create a mock docker provisioner
	dockerProvisioner := provisioners.NewDockerProvisioner(provisioners.DockerProvisionerConfig{
		ContainerRuntime: mockRuntime,
		Networks:         []string{},
		Verbose:          false,
		Static:           false,
		BlockNetwork:     false,
		Cpus:             1,
		Memory:           "512m",
		LongLived:        false,
	})

	// Create provisioner map
	provisionerMap := make(map[provisioners.ProvisionerType]provisioners.Provisioner)
	provisionerMap[provisioners.DockerProvisioner] = dockerProvisioner

	cp := newClientPool(ClientPoolConfig{
		ContainerRuntime:   mockRuntime,
		Provisioners:       provisionerMap,
		DefaultProvisioner: provisioners.DockerProvisioner,
	})

	tool := catalog.Tool{
		Name: "test-tool",
		Container: catalog.Container{
			Image: "alpine:latest",
		},
	}

	params := &mcp.CallToolParams{
		Name:      "test-tool",
		Arguments: map[string]any{},
	}

	// Execute the tool
	result, err := cp.runToolContainer(context.Background(), tool, params)
	// Should not return an error (errors are handled as tool results)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should return an error result
	if !result.IsError {
		t.Error("Expected error result")
	}

	textContent := result.Content[0].(*mcp.TextContent)
	if !containsString(textContent.Text, "Container runtime error") {
		t.Errorf("Expected error message to contain 'Container runtime error', got: %s", textContent.Text)
	}
}

func TestRunToolContainer_HandlesContainerFailure(t *testing.T) {
	// Create a mock container runtime that returns a failed container execution
	mockRuntime := &mockContainerRuntime{
		resultToReturn: &runtime.ContainerResult{
			Stdout:   "some output",
			Stderr:   "error message",
			ExitCode: 1,
			Success:  false,
			Runtime:  "mock",
		},
	}

	// Create a mock docker provisioner
	dockerProvisioner := provisioners.NewDockerProvisioner(provisioners.DockerProvisionerConfig{
		ContainerRuntime: mockRuntime,
		Networks:         []string{},
		Verbose:          false,
		Static:           false,
		BlockNetwork:     false,
		Cpus:             1,
		Memory:           "512m",
		LongLived:        false,
	})

	// Create provisioner map
	provisionerMap := make(map[provisioners.ProvisionerType]provisioners.Provisioner)
	provisionerMap[provisioners.DockerProvisioner] = dockerProvisioner

	cp := newClientPool(ClientPoolConfig{
		ContainerRuntime:   mockRuntime,
		Provisioners:       provisionerMap,
		DefaultProvisioner: provisioners.DockerProvisioner,
	})

	tool := catalog.Tool{
		Name: "test-tool",
		Container: catalog.Container{
			Image: "alpine:latest",
		},
	}

	params := &mcp.CallToolParams{
		Name:      "test-tool",
		Arguments: map[string]any{},
	}

	// Execute the tool
	result, err := cp.runToolContainer(context.Background(), tool, params)
	// Should not return an error (container failures are handled as tool results)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should return an error result indicating container failure
	if !result.IsError {
		t.Error("Expected error result for failed container")
	}

	// Should still contain the stdout from the container
	textContent := result.Content[0].(*mcp.TextContent)
	if textContent.Text != "some output" {
		t.Errorf("Expected stdout 'some output', got '%s'", textContent.Text)
	}
}

// Helper types for testing
type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}

// Helper function
func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		haystack[:len(needle)] == needle ||
		len(haystack) > len(needle) &&
			haystack[len(haystack)-len(needle):] == needle ||
		containsSubstring(haystack, needle)
}

func containsSubstring(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestMockContainerRuntime_StartContainer_Success(t *testing.T) {
	mockRuntime := &mockContainerRuntime{}

	spec := runtime.ContainerSpec{
		Name:    "test-container",
		Image:   "nginx:latest",
		Command: []string{"nginx", "-g", "daemon off;"},
		Env: map[string]string{
			"ENV_VAR": "test-value",
		},
	}

	ctx := context.Background()
	handle, err := mockRuntime.StartContainer(ctx, spec)

	// Verify success
	require.NoError(t, err)
	assert.NotNil(t, handle)

	// Verify handle properties
	assert.Equal(t, "mock-container-1", handle.ID)
	assert.NotNil(t, handle.Stdin)
	assert.NotNil(t, handle.Stdout)

	// Verify mock state
	assert.Equal(t, 1, mockRuntime.startContainerCalled)
	assert.Equal(t, &spec, mockRuntime.capturedStartSpec)
}

func TestMockContainerRuntime_StartContainer_Failure(t *testing.T) {
	expectedError := fmt.Errorf("container runtime unavailable")
	mockRuntime := &mockContainerRuntime{
		startErrorToReturn: expectedError,
	}

	spec := runtime.ContainerSpec{
		Name:  "failing-container",
		Image: "nginx:latest",
	}

	ctx := context.Background()
	handle, err := mockRuntime.StartContainer(ctx, spec)

	// Verify failure
	require.Error(t, err)
	assert.Equal(t, expectedError, err)
	assert.Nil(t, handle)

	// Verify mock state
	assert.Equal(t, 1, mockRuntime.startContainerCalled)
	assert.Equal(t, &spec, mockRuntime.capturedStartSpec)
}

func TestMockContainerRuntime_StartContainer_CustomHandle(t *testing.T) {
	customStdin := &mockReadWriteCloser{}
	customStdout := &mockReadWriteCloser{}
	customHandle := &runtime.ContainerHandle{
		ID:     "custom-container-id",
		Stdin:  customStdin,
		Stdout: customStdout,
	}

	mockRuntime := &mockContainerRuntime{
		startHandleToReturn: customHandle,
	}

	spec := runtime.ContainerSpec{
		Name:  "custom-container",
		Image: "alpine:latest",
	}

	ctx := context.Background()
	handle, err := mockRuntime.StartContainer(ctx, spec)

	// Verify custom handle returned
	require.NoError(t, err)
	assert.Equal(t, customHandle, handle)
	assert.Equal(t, "custom-container-id", handle.ID)
	assert.Equal(t, customStdin, handle.Stdin)
	assert.Equal(t, customStdout, handle.Stdout)
}

func TestMockContainerRuntime_StopContainer_Variations(t *testing.T) {
	tests := []struct {
		name          string
		handle        *runtime.ContainerHandle
		expectedError error
		shouldSucceed bool
	}{
		{
			name: "Stop container success",
			handle: &runtime.ContainerHandle{
				ID:     "container-to-stop",
				Stdin:  &mockReadWriteCloser{},
				Stdout: &mockReadWriteCloser{},
			},
			expectedError: nil,
			shouldSucceed: true,
		},
		{
			name: "Stop container failure",
			handle: &runtime.ContainerHandle{
				ID:     "failing-container",
				Stdin:  &mockReadWriteCloser{},
				Stdout: &mockReadWriteCloser{},
			},
			expectedError: fmt.Errorf("failed to stop container"),
			shouldSucceed: false,
		},
		{
			name:          "Stop nil handle",
			handle:        nil,
			expectedError: nil,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRuntime := &mockContainerRuntime{
				stopErrorToReturn: tt.expectedError,
			}

			ctx := context.Background()
			err := mockRuntime.StopContainer(ctx, tt.handle)

			if tt.shouldSucceed {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			}

			// Verify mock state
			assert.Equal(t, 1, mockRuntime.stopContainerCalled)
			assert.Equal(t, tt.handle, mockRuntime.capturedStopHandle)
		})
	}
}

func TestContainerRuntime_SpecValidation(t *testing.T) {
	mockRuntime := &mockContainerRuntime{}

	tests := []struct {
		name            string
		spec            runtime.ContainerSpec
		expectError     bool
		expectedCapture bool
	}{
		{
			name: "Valid complete spec",
			spec: runtime.ContainerSpec{
				Name:    "valid-container",
				Image:   "nginx:1.21",
				Command: []string{"nginx", "-g", "daemon off;"},
				Env: map[string]string{
					"ENV1": "value1",
					"ENV2": "value2",
				},
				Volumes:        []string{"/host:/container"},
				Networks:       []string{"test-network"},
				CPUs:           2,
				Memory:         "1g",
				Persistent:     true,
				AttachStdio:    true,
				KeepStdinOpen:  true,
				RestartPolicy:  "no",
				RemoveAfterRun: false,
				Interactive:    true,
				Init:           false,
				Privileged:     false,
				DisableNetwork: false,
			},
			expectError:     false,
			expectedCapture: true,
		},
		{
			name: "Minimal valid spec",
			spec: runtime.ContainerSpec{
				Name:  "minimal",
				Image: "alpine",
			},
			expectError:     false,
			expectedCapture: true,
		},
		{
			name: "Spec with error",
			spec: runtime.ContainerSpec{
				Name:  "error-container",
				Image: "should-fail",
			},
			expectError:     true,
			expectedCapture: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock state
			mockRuntime.startContainerCalled = 0
			mockRuntime.capturedStartSpec = nil

			if tt.expectError {
				mockRuntime.startErrorToReturn = fmt.Errorf("test error for %s", tt.spec.Name)
			} else {
				mockRuntime.startErrorToReturn = nil
			}

			ctx := context.Background()
			handle, err := mockRuntime.StartContainer(ctx, tt.spec)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, handle)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, handle)
			}

			if tt.expectedCapture {
				assert.Equal(t, 1, mockRuntime.startContainerCalled)
				assert.Equal(t, &tt.spec, mockRuntime.capturedStartSpec)

				// Verify the spec was captured correctly
				capturedSpec := mockRuntime.capturedStartSpec
				assert.Equal(t, tt.spec.Name, capturedSpec.Name)
				assert.Equal(t, tt.spec.Image, capturedSpec.Image)
				assert.Equal(t, tt.spec.Command, capturedSpec.Command)
				assert.Equal(t, tt.spec.Env, capturedSpec.Env)
				assert.Equal(t, tt.spec.Volumes, capturedSpec.Volumes)
				assert.Equal(t, tt.spec.Networks, capturedSpec.Networks)
			}
		})
	}
}

func TestMockContainerRuntime_MultipleOperations(t *testing.T) {
	mockRuntime := &mockContainerRuntime{}

	// Test multiple start operations
	ctx := context.Background()

	spec1 := runtime.ContainerSpec{Name: "container-1", Image: "nginx:latest"}
	handle1, err1 := mockRuntime.StartContainer(ctx, spec1)
	require.NoError(t, err1)
	assert.Equal(t, "mock-container-1", handle1.ID)

	spec2 := runtime.ContainerSpec{Name: "container-2", Image: "redis:latest"}
	handle2, err2 := mockRuntime.StartContainer(ctx, spec2)
	require.NoError(t, err2)
	assert.Equal(t, "mock-container-2", handle2.ID)

	// Verify call counts
	assert.Equal(t, 2, mockRuntime.startContainerCalled)

	// Test stop operations
	err := mockRuntime.StopContainer(ctx, handle1)
	require.NoError(t, err)

	err = mockRuntime.StopContainer(ctx, handle2)
	require.NoError(t, err)

	// Verify stop counts
	assert.Equal(t, 2, mockRuntime.stopContainerCalled)

	// Test run operation
	runSpec := runtime.ContainerSpec{Name: "run-container", Image: "alpine:latest"}
	result, runErr := mockRuntime.RunContainer(ctx, runSpec)
	require.NoError(t, runErr)
	assert.NotNil(t, result)
	assert.Equal(t, 1, mockRuntime.runContainerCalled)

	// Test shutdown
	shutdownErr := mockRuntime.Shutdown(ctx)
	require.NoError(t, shutdownErr)
	assert.Equal(t, 1, mockRuntime.shutdownCalled)
}
