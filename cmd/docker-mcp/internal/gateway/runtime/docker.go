package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// DockerContainerRuntime implements ContainerRuntime using Docker CLI
type DockerContainerRuntime struct {
	config DockerContainerRuntimeConfig
}

// DockerContainerRuntimeConfig holds Docker-specific configuration
type DockerContainerRuntimeConfig struct {
	ContainerRuntimeConfig // Embedded base config

	// Docker-specific options
	PullPolicy    string // Docker pull policy (e.g., "never", "always", "missing")
	DockerContext string // Docker context to use for all Docker commands
}

// NewDockerContainerRuntime creates a new Docker container runtime
func NewDockerContainerRuntime(config DockerContainerRuntimeConfig) *DockerContainerRuntime {
	return &DockerContainerRuntime{
		config: config,
	}
}

// GetName returns the runtime identifier
func (dcr *DockerContainerRuntime) GetName() string {
	return "docker"
}

// buildDockerCommand creates a Docker command with context support
func (dcr *DockerContainerRuntime) buildDockerCommand(ctx context.Context, args ...string) *exec.Cmd {
	dockerArgs := []string{}

	// Add context if specified
	if dcr.config.DockerContext != "" {
		dockerArgs = append(dockerArgs, "--context", dcr.config.DockerContext)
	}

	// Add the rest of the arguments
	dockerArgs = append(dockerArgs, args...)

	return exec.CommandContext(ctx, "docker", dockerArgs...)
}

// RunContainer executes a container using Docker CLI and returns the result
func (dcr *DockerContainerRuntime) RunContainer(ctx context.Context, spec ContainerSpec) (*ContainerResult, error) {
	// Force a log output regardless of verbose setting to confirm this method is called
	dcr.debugLog("RunContainer ENTRY for POCI tool:", spec.Name)

	// Build Docker arguments
	args := dcr.buildDockerArgs(spec)

	// Log the container execution if verbose
	if dcr.config.Verbose {
		fmt.Fprintf(os.Stderr, "[DockerContainerRuntime] Running container %s with args: %v\n", spec.Image, args)
	}

	// Execute Docker command with context support
	cmd := dcr.buildDockerCommand(ctx, args...)

	// Capture stderr if verbose mode is enabled
	if dcr.config.Verbose {
		cmd.Stderr = os.Stderr
	}

	// Execute and capture output
	stdout, err := cmd.Output()

	// Determine success and exit code
	success := err == nil
	exitCode := 0
	stderr := ""

	if err != nil {
		// Try to extract exit code from error
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
			stderr = string(exitError.Stderr)
		} else {
			// Non-exit error (e.g., command not found, context canceled)
			exitCode = -1
			stderr = err.Error()
		}
	}

	return &ContainerResult{
		Stdout:      string(stdout),
		Stderr:      stderr,
		ExitCode:    exitCode,
		Success:     success,
		ContainerID: "", // Docker CLI doesn't easily provide container ID for run commands
		Runtime:     dcr.GetName(),
	}, nil
}

// StartContainer starts a persistent container and returns handles for ongoing communication
func (dcr *DockerContainerRuntime) StartContainer(ctx context.Context, spec ContainerSpec) (*ContainerHandle, error) {
	// Build Docker arguments for persistent container
	args := dcr.buildDockerArgsForPersistent(spec)

	// Log the container execution if verbose
	if dcr.config.Verbose {
		fmt.Fprintf(os.Stderr, "[DockerContainerRuntime] Starting persistent container %s with args: %v\n", spec.Image, args)
	}

	// Create Docker command with stdio pipes and context support
	cmd := dcr.buildDockerCommand(ctx, args...)

	// Create stdio pipes for MCP communication
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the container
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Create cleanup function
	cleanup := func() error {
		// Close pipes
		stdin.Close()
		stdout.Close()
		stderr.Close()

		// Wait for container to exit (with timeout)
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			// Container didn't exit gracefully, force kill
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return <-done
		}
	}

	return &ContainerHandle{
		ID:      "", // Docker CLI doesn't easily provide container ID
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		Cleanup: cleanup,
	}, nil
}

// StopContainer stops a persistent container and cleans up resources
func (dcr *DockerContainerRuntime) StopContainer(_ context.Context, handle *ContainerHandle) error {
	if handle.Cleanup != nil {
		return handle.Cleanup()
	}
	return nil
}

// buildDockerArgs constructs Docker CLI arguments from a ContainerSpec
func (dcr *DockerContainerRuntime) buildDockerArgs(spec ContainerSpec) []string {
	args := []string{"run"}

	// Basic container behavior
	if spec.RemoveAfterRun {
		args = append(args, "--rm")
	}
	if spec.Interactive {
		args = append(args, "-i")
	}
	if spec.Init {
		args = append(args, "--init")
	}
	if spec.Privileged {
		args = append(args, "--privileged")
	}

	// Security options (always apply for security)
	args = append(args, "--security-opt", "no-new-privileges")

	// Resource limits
	if spec.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", spec.CPUs))
	}
	if spec.Memory != "" {
		args = append(args, "--memory", spec.Memory)
	}

	// Network configuration
	if spec.DisableNetwork {
		args = append(args, "--network", "none")
	} else {
		for _, network := range spec.Networks {
			args = append(args, "--network", network)
		}
	}

	// Docker-specific proxy configuration
	for _, link := range spec.Links {
		if link != "" {
			args = append(args, "--link", link)
		}
	}

	if spec.DNS != "" {
		args = append(args, "--dns", spec.DNS)
	}

	// Environment variables
	for name, value := range spec.Env {
		if value != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", name, value))
		} else {
			// Pass through environment variable from host
			args = append(args, "-e", name)
		}
	}

	// Volume mounts
	for _, volume := range spec.Volumes {
		if volume != "" {
			args = append(args, "-v", volume)
		}
	}

	// User specification
	if spec.User != "" {
		args = append(args, "-u", spec.User)
	}

	// Pull policy
	pullPolicy := dcr.config.PullPolicy
	if pullPolicy == "" {
		pullPolicy = "never" // Default to never pull for consistency with existing behavior
	}
	args = append(args, "--pull", pullPolicy)

	// Labels for identification (following existing pattern)
	if spec.Name != "" {
		args = append(args,
			"-l", "docker-mcp=true",
			"-l", "docker-mcp-tool-type=container-tool",
			"-l", "docker-mcp-name="+spec.Name,
			"-l", "docker-mcp-transport=ephemeral",
		)
	}

	// Custom labels from spec
	for key, value := range spec.Labels {
		args = append(args, "-l", fmt.Sprintf("%s=%s", key, value))
	}

	// Container image
	args = append(args, spec.Image)

	// Command to run inside container
	args = append(args, spec.Command...)

	return args
}

// buildDockerArgsForPersistent constructs Docker CLI arguments for persistent containers
func (dcr *DockerContainerRuntime) buildDockerArgsForPersistent(spec ContainerSpec) []string {
	args := []string{"run"}

	// Persistent container behavior - never use --rm, always interactive with stdin
	args = append(args, "-i") // Always interactive for MCP communication

	if spec.Init {
		args = append(args, "--init")
	}
	if spec.Privileged {
		args = append(args, "--privileged")
	}

	// Security options (always apply for security)
	args = append(args, "--security-opt", "no-new-privileges")

	// Resource limits
	if spec.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", spec.CPUs))
	}
	if spec.Memory != "" {
		args = append(args, "--memory", spec.Memory)
	}

	// Network configuration
	if spec.DisableNetwork {
		args = append(args, "--network", "none")
	} else {
		for _, network := range spec.Networks {
			args = append(args, "--network", network)
		}
	}

	// Docker-specific proxy configuration
	for _, link := range spec.Links {
		if link != "" {
			args = append(args, "--link", link)
		}
	}

	if spec.DNS != "" {
		args = append(args, "--dns", spec.DNS)
	}

	// Environment variables
	for name, value := range spec.Env {
		if value != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", name, value))
		} else {
			// Pass through environment variable from host
			args = append(args, "-e", name)
		}
	}

	// Volume mounts
	for _, volume := range spec.Volumes {
		if volume != "" {
			args = append(args, "-v", volume)
		}
	}

	// User specification
	if spec.User != "" {
		args = append(args, "-u", spec.User)
	}

	// Pull policy
	pullPolicy := dcr.config.PullPolicy
	if pullPolicy == "" {
		pullPolicy = "never" // Default to never pull for consistency with existing behavior
	}
	args = append(args, "--pull", pullPolicy)

	// Labels for identification (MCP server pattern)
	if spec.Name != "" {
		args = append(args,
			"-l", "docker-mcp=true",
			"-l", "docker-mcp-tool-type=mcp",
			"-l", "docker-mcp-name="+spec.Name,
			"-l", "docker-mcp-transport=stdio",
		)
	}

	// Custom labels from spec
	for key, value := range spec.Labels {
		args = append(args, "-l", fmt.Sprintf("%s=%s", key, value))
	}

	// Container image
	args = append(args, spec.Image)

	// Command to run inside container
	args = append(args, spec.Command...)

	return args
}

// Shutdown performs cleanup of all resources managed by this runtime
func (dcr *DockerContainerRuntime) Shutdown(_ context.Context) error {
	// For Docker runtime, shutdown is mainly handled by individual container cleanup
	// The runtime itself doesn't maintain long-lived resources beyond individual containers
	// which are managed by their respective cleanup functions

	return nil
}

// debugLog prints debug messages only when verbose mode is enabled
func (dcr *DockerContainerRuntime) debugLog(args ...any) {
	if dcr.config.Verbose {
		prefixedArgs := append([]any{"[DockerContainerRuntime]"}, args...)
		fmt.Fprintln(os.Stderr, prefixedArgs...)
	}
}
