package runtime

import (
	"context"
	"io"
)

// ContainerRuntime provides container execution capabilities.
// This abstraction handles ALL container operations: ephemeral (container tools)
// and persistent (MCP servers), with the lifecycle determined by ContainerSpec configuration.
type ContainerRuntime interface {
	// RunContainer executes a container and returns its output.
	// This is for ephemeral, synchronous operations (container tools).
	RunContainer(ctx context.Context, spec ContainerSpec) (*ContainerResult, error)

	// StartContainer starts a persistent container and returns handles for ongoing communication.
	// This is for long-lived operations (MCP servers) that need stdin/stdout streams.
	StartContainer(ctx context.Context, spec ContainerSpec) (*ContainerHandle, error)

	// StopContainer stops a persistent container and cleans up resources.
	StopContainer(ctx context.Context, handle *ContainerHandle) error

	// GetName returns the runtime implementation name for identification
	GetName() string

	// Shutdown performs cleanup of all resources managed by this runtime.
	// This is called during gateway shutdown to ensure proper resource cleanup.
	Shutdown(ctx context.Context) error
}

// ContainerHandle provides access to a persistent container's resources
type ContainerHandle struct {
	ID      string         // Container/Pod ID for management operations
	Stdin   io.WriteCloser // MCP protocol input stream
	Stdout  io.ReadCloser  // MCP protocol output stream
	Stderr  io.ReadCloser  // Error/debug output stream
	Cleanup func() error   // Runtime-specific cleanup function
}

// ContainerSpec defines the configuration for running a container
type ContainerSpec struct {
	// Basic container configuration
	Name    string   // Container name/identifier for logging
	Image   string   // Container image (e.g., "alpine:latest")
	Command []string // Command to run inside container

	// Runtime configuration
	Networks []string          // Docker networks to attach to
	Volumes  []string          // Volume mounts (e.g., "/host:/container")
	Env      map[string]string // Environment variables
	User     string            // User to run as (e.g., "1000:1000", "user:group")
	Labels   map[string]string // Container/Pod labels for identification and management

	// Resource limits
	CPUs   int    // CPU limit
	Memory string // Memory limit (e.g., "512m", "1g")

	// Lifecycle behavior
	Persistent     bool   // false = ephemeral (--rm), true = long-lived
	AttachStdio    bool   // true = attach stdin/stdout for MCP communication
	KeepStdinOpen  bool   // Keep stdin open for ongoing communication (MCP protocol)
	RestartPolicy  string // Container restart behavior ("no", "always", "on-failure")
	StartupTimeout int    // Maximum time in seconds to wait for container to be ready (0 = use runtime default)

	// Security and behavior (for ephemeral containers)
	RemoveAfterRun bool // Whether to remove container after execution (--rm) - ignored if Persistent=true
	Interactive    bool // Whether to keep stdin open (-i)
	Init           bool // Whether to run init process (--init)
	Privileged     bool // Whether to run in privileged mode

	// Network isolation
	DisableNetwork bool // Whether to disable networking (--network none)

	// Docker-specific network proxy configuration (ignored by other runtimes)
	Links []string // Container links for hostname resolution (--link proxy:hostname)
	DNS   string   // DNS server override (--dns 127.0.0.11)

	// Kubernetes-specific configuration
	SecretKeyRefs map[string]SecretKeyRef // Environment variables from Kubernetes Secrets
	ConfigMapRefs []string                // ConfigMaps to load as environment variables (envFrom)
}

// SecretKeyRef represents a Kubernetes secretKeyRef for environment variables
type SecretKeyRef struct {
	Name string // Secret resource name
	Key  string // Key within the secret data
}

// ContainerResult contains the output and status from container execution
type ContainerResult struct {
	// Output
	Stdout string // Standard output
	Stderr string // Standard error (if captured)

	// Execution status
	ExitCode int  // Container exit code
	Success  bool // Whether execution was successful (ExitCode == 0)

	// Runtime information
	ContainerID string // Container ID (if available)
	Runtime     string // Runtime that executed this container
}

// ContainerRuntimeConfig holds configuration for creating container runtimes
type ContainerRuntimeConfig struct {
	// Common configuration that all runtimes might need
	Verbose bool // Whether to enable verbose logging

	// Runtime-specific configuration can be added by implementations
	// For example, DockerContainerRuntimeConfig would embed this and add Docker-specific fields
}
