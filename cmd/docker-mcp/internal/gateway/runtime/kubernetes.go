package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// KubernetesContainerRuntime implements ContainerRuntime using client-go
type KubernetesContainerRuntime struct {
	namespace   string
	kubeconfig  string
	kubeContext string
	verbose     bool
	clientset   kubernetes.Interface
	restConfig  *rest.Config
}

// KubernetesContainerRuntimeConfig holds Kubernetes-specific configuration
type KubernetesContainerRuntimeConfig struct {
	ContainerRuntimeConfig // Embedded common configuration

	// Kubernetes-specific configuration
	Namespace   string // Kubernetes namespace to deploy to
	Kubeconfig  string // Path to kubeconfig file (empty = use default)
	KubeContext string // Kubernetes context to use (empty = use current context)
}

// NewKubernetesContainerRuntime creates a new Kubernetes container runtime with automatic in-cluster/out-of-cluster detection
func NewKubernetesContainerRuntime(config KubernetesContainerRuntimeConfig) (*KubernetesContainerRuntime, error) {
	// Use default namespace if not specified
	namespace := config.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Try to get Kubernetes configuration
	restConfig, err := getKubernetesConfig(config.Kubeconfig, config.KubeContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	// Create clientset from config
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	runtime := &KubernetesContainerRuntime{
		namespace:   namespace,
		kubeconfig:  config.Kubeconfig,
		kubeContext: config.KubeContext,
		verbose:     config.Verbose,
		clientset:   clientset,
		restConfig:  restConfig,
	}

	return runtime, nil
}

// getKubernetesConfig returns Kubernetes config with automatic in-cluster/out-of-cluster detection
// Following patterns from client-go examples
func getKubernetesConfig(kubeconfig, _ string) (*rest.Config, error) {
	// Try in-cluster config first (for pods running in Kubernetes)
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}

	// Fall back to out-of-cluster config (for local/external execution)
	kubeconfigPath := kubeconfig
	if kubeconfigPath == "" {
		// Default to ~/.kube/config like the examples
		if home := homedir.HomeDir(); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	// Use clientcmd.BuildConfigFromFlags() like the out-of-cluster example
	// This handles context selection if we extend it later
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	// TODO: Add context override support if kubeContext is specified
	// For now, use the current context from kubeconfig

	return config, nil
}

// GetName returns the runtime implementation name
func (k *KubernetesContainerRuntime) GetName() string {
	return "kubernetes"
}

// GetClientset returns the Kubernetes clientset for cleanup operations
func (k *KubernetesContainerRuntime) GetClientset() (kubernetes.Interface, error) {
	if k.clientset == nil {
		return nil, fmt.Errorf("kubernetes clientset not initialized")
	}
	return k.clientset, nil
}

// GetNamespace returns the current namespace
func (k *KubernetesContainerRuntime) GetNamespace() string {
	return k.namespace
}

// Shutdown performs cleanup of all resources managed by this runtime
func (k *KubernetesContainerRuntime) Shutdown(_ context.Context) error {
	k.debugLog("Shutting down Kubernetes container runtime")

	// For Kubernetes runtime, shutdown is mainly handled by the provisioner
	// The runtime itself doesn't maintain long-lived resources beyond individual pods
	// which are managed by their respective provisioners

	k.debugLog("Kubernetes container runtime shutdown completed")
	return nil
}

// RunContainer executes a container as a Kubernetes Pod with separate stdout/stderr streams
// This is for ephemeral, synchronous operations (container tools)
// Uses the same Pod + attach approach as StartContainer, but waits for completion and returns output
func (k *KubernetesContainerRuntime) RunContainer(ctx context.Context, spec ContainerSpec) (*ContainerResult, error) {
	k.debugLog("RunContainer ENTRY for POCI tool:", spec.Name)
	k.debugLog("=== RunContainer ENTRY for POCI tool:", spec.Name, "===")
	k.debugLog("  - Image:", spec.Image)
	k.debugLog("  - Command:", spec.Command)
	k.debugLog("Using ENTRYPOINT override + exec-based sidecar approach for stdout/stderr separation")

	// Create Pod with ENTRYPOINT override and sidecar container
	k.debugLog("STEP 1: Creating Pod with ENTRYPOINT override and sidecar...")
	podName, cleanup, err := k.createSidecarPod(ctx, spec)
	if err != nil {
		k.debugLog("ERROR: createSidecarPod failed for POCI tool:", err)
		return nil, fmt.Errorf("failed to create sidecar pod: %w", err)
	}
	k.debugLog("SUCCESS: createSidecarPod returned Pod name:", podName)

	// Ensure cleanup happens regardless of success/failure
	defer func() {
		k.debugLog("CLEANUP: Starting cleanup for Pod:", podName)
		if cleanupErr := cleanup(); cleanupErr != nil {
			k.debugLog("Warning: Error during container cleanup:", cleanupErr)
		} else {
			k.debugLog("SUCCESS: Cleanup completed for Pod:", podName)
		}
	}()

	// Wait for main container completion to get exit code
	k.debugLog("STEP 2: Waiting for main container completion...")
	exitCode, err := k.waitForPodCompletion(ctx, podName)
	if err != nil {
		k.debugLog("ERROR: waitForPodCompletion failed:", err)
		return nil, fmt.Errorf("failed to wait for Pod completion: %w", err)
	}

	k.debugLog("SUCCESS: Main container completed with exit code:", exitCode)

	// Wait for completion marker to ensure logs are fully written
	k.debugLog("STEP 3: Waiting for completion marker to ensure logs are complete...")
	err = k.waitForCompletionMarker(ctx, podName)
	if err != nil {
		k.debugLog("ERROR: Failed to wait for completion marker:", err)
		return nil, fmt.Errorf("failed to wait for completion marker: %w", err)
	}
	k.debugLog("SUCCESS: Completion marker found, logs are complete")

	// Now exec into sidecar to read the log files (no timing issues!)
	k.debugLog("STEP 4: Using exec to read stdout/stderr files from sidecar...")

	// Read stdout file via exec
	k.debugLog("Exec: Reading stdout file...")
	stdoutBytes, err := k.execInContainer(ctx, podName, "sidecar", []string{"cat", "/logs/stdout.log"})
	if err != nil {
		k.debugLog("ERROR: Failed to read stdout file via exec:", err)
		return nil, fmt.Errorf("failed to read stdout file: %w", err)
	}
	k.debugLog("SUCCESS: Read stdout file, length:", len(stdoutBytes))

	// Read stderr file via exec
	k.debugLog("Exec: Reading stderr file...")
	stderrBytes, err := k.execInContainer(ctx, podName, "sidecar", []string{"cat", "/logs/stderr.log"})
	if err != nil {
		k.debugLog("ERROR: Failed to read stderr file via exec:", err)
		return nil, fmt.Errorf("failed to read stderr file: %w", err)
	}
	k.debugLog("SUCCESS: Read stderr file, length:", len(stderrBytes))

	success := exitCode == 0
	k.debugLog("SUCCESS: POCI tool execution completed with separated streams!")
	k.debugLog("Pod name:", podName)
	k.debugLog("Exit code:", exitCode)
	k.debugLog("Success:", success)
	k.debugLog("Stdout length:", len(stdoutBytes))
	k.debugLog("Stderr length:", len(stderrBytes))

	return &ContainerResult{
		Stdout:      string(stdoutBytes),
		Stderr:      string(stderrBytes),
		ExitCode:    exitCode,
		Success:     success,
		ContainerID: podName,
		Runtime:     k.GetName(),
	}, nil
}

// StartContainer starts a persistent container as a Kubernetes Pod and returns handles
// This is for long-lived operations (MCP servers) that need stdin/stdout streams
func (k *KubernetesContainerRuntime) StartContainer(ctx context.Context, spec ContainerSpec) (*ContainerHandle, error) {
	k.debugLog("StartContainer called for persistent container:", spec.Name)

	// Use shared helper with waitForReady=true for persistent MCP servers
	return k.createPodWithStreams(ctx, spec, true)
}

// createPodWithStreams is a shared helper that creates a Pod with attach streams
// waitForReady controls whether to wait for Pod Ready state (true for MCP servers, false for POCI tools)
func (k *KubernetesContainerRuntime) createPodWithStreams(ctx context.Context, spec ContainerSpec, waitForReady bool) (*ContainerHandle, error) {
	// Generate unique Pod name with timestamp to avoid conflicts
	podName := fmt.Sprintf("mcp-%s-%d", sanitizeName(spec.Name), time.Now().Unix())

	// Create Pod manifest using client-go objects
	pod := k.createPodManifest(podName, spec)

	k.debugLog("Creating Pod:", podName, "in namespace:", k.namespace)

	// Create Pod using client-go
	createdPod, err := k.clientset.CoreV1().Pods(k.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Pod: %w", err)
	}

	k.debugLog("Pod created successfully:", createdPod.Name)

	// Conditionally wait for Pod to be ready (only for persistent MCP servers)
	if waitForReady {
		k.debugLog("Waiting for Pod to be ready (persistent container)...")
		if err := k.waitForPodReady(ctx, podName, spec.StartupTimeout); err != nil {
			// Clean up Pod if wait fails
			deletePolicy := metav1.DeletePropagationForeground
			_ = k.clientset.CoreV1().Pods(k.namespace).Delete(ctx, podName, metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			})
			return nil, fmt.Errorf("pod failed to become ready: %w", err)
		}
		k.debugLog("Pod is ready:", podName)
	} else {
		k.debugLog("Skipping ready wait (ephemeral container) - proceeding to attach")
	}

	// Create real stdio streams using Pod attach API (not exec API)
	k.debugLog("Creating attach streams for Pod:", podName)
	stdin, stdout, cleanup, err := k.createPodAttachStreams(ctx, podName)
	if err != nil {
		k.debugLog("ERROR: Failed to create attach streams:", err)
		// Clean up Pod if attach fails
		deletePolicy := metav1.DeletePropagationForeground
		_ = k.clientset.CoreV1().Pods(k.namespace).Delete(ctx, podName, metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		})
		return nil, fmt.Errorf("failed to attach to Pod stdio: %w", err)
	}
	k.debugLog("SUCCESS: Attach streams created")

	return &ContainerHandle{
		ID:      podName,
		Stdin:   stdin,
		Stdout:  stdout,
		Cleanup: cleanup,
	}, nil
}

// createSidecarPod creates a Pod with main container (ENTRYPOINT override) + sidecar container for log access
func (k *KubernetesContainerRuntime) createSidecarPod(ctx context.Context, spec ContainerSpec) (string, func() error, error) {
	// Generate unique Pod name with timestamp to avoid conflicts
	podName := fmt.Sprintf("mcp-%s-%d", sanitizeName(spec.Name), time.Now().Unix())

	k.debugLog("Creating sidecar Pod:", podName, "in namespace:", k.namespace)
	k.debugLog("Original command:", spec.Command)

	// Build shell command that redirects stdout/stderr to separate files
	// We need to reconstruct what would normally run: ENTRYPOINT + CMD + our args
	// Since Kubernetes would normally append spec.Command to the image's ENTRYPOINT,
	// we need to get the original ENTRYPOINT and reconstruct the full command

	k.debugLog("Inspecting image to get ENTRYPOINT and CMD:", spec.Image)
	k.debugLog("Starting image inspection for:", spec.Image)

	entrypoint, cmd, err := k.inspectImage(ctx, spec.Image)
	if err != nil {
		k.debugLog("ERROR: Image inspection failed:", err)
		k.debugLog("ERROR: Failed to inspect image:", err)
		return "", nil, fmt.Errorf("failed to inspect image %s: %w", spec.Image, err)
	}

	fmt.Printf("[KubernetesRuntime] Image inspection completed successfully\n")

	k.debugLog("Image inspection results:")
	k.debugLog("  ENTRYPOINT:", entrypoint)
	k.debugLog("  CMD:", cmd)
	k.debugLog("  spec.Command (our args):", spec.Command)

	k.debugLog("Starting command reconstruction...")

	// Reconstruct the full command that would normally run
	// Docker/Kubernetes behavior: ENTRYPOINT + (CMD + our_args OR just our_args if we provide any)
	var fullCommand []string

	// Start with ENTRYPOINT
	fullCommand = append(fullCommand, entrypoint...)

	// Add CMD + our args, or just our args if we provide any
	if len(spec.Command) > 0 {
		// If we provide args, they replace CMD entirely
		fullCommand = append(fullCommand, spec.Command...)
	} else if len(cmd) > 0 {
		// If we don't provide args, use the image's CMD
		fullCommand = append(fullCommand, cmd...)
	}

	// Convert to single shell command string for redirection + completion marker
	commandStr := strings.Join(fullCommand, " ")
	wrappedCommand := fmt.Sprintf("%s >/logs/stdout.log 2>/logs/stderr.log; echo $? > /logs/exit_code.log; touch /logs/complete.marker", commandStr)

	k.debugLog("Command reconstruction complete:")
	k.debugLog("  Full command:", fullCommand)
	k.debugLog("  Command string:", commandStr)
	k.debugLog("  Wrapped command:", wrappedCommand)

	k.debugLog("Reconstructed full command:", fullCommand)
	k.debugLog("Command string:", commandStr)
	k.debugLog("Wrapped command:", wrappedCommand)

	// Create Pod manifest with main container + sidecar
	k.debugLog("Creating Pod manifest...")
	pod := k.createSidecarPodManifest(podName, spec, wrappedCommand)

	// Create Pod using client-go
	k.debugLog("Creating Pod:", podName)
	createdPod, err := k.clientset.CoreV1().Pods(k.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		k.debugLog("ERROR: Failed to create Pod:", err)
		return "", nil, fmt.Errorf("failed to create sidecar Pod: %w", err)
	}

	k.debugLog("Pod created successfully:", createdPod.Name)
	k.debugLog("Sidecar Pod created successfully:", createdPod.Name)

	// Create cleanup function
	cleanup := func() error {
		k.debugLog("Cleaning up sidecar Pod:", podName)

		deletePolicy := metav1.DeletePropagationForeground
		if err := k.clientset.CoreV1().Pods(k.namespace).Delete(ctx, podName, metav1.DeleteOptions{
			PropagationPolicy:  &deletePolicy,
			GracePeriodSeconds: &[]int64{0}[0], // Immediate termination
		}); err != nil && !errors.IsNotFound(err) {
			k.debugLog("Warning: Failed to delete sidecar Pod:", err)
			return fmt.Errorf("failed to delete sidecar Pod: %w", err)
		}

		k.debugLog("Sidecar Pod cleaned up successfully:", podName)
		return nil
	}

	return podName, cleanup, nil
}

// execInContainer executes a command inside a specific container and returns the output
func (k *KubernetesContainerRuntime) execInContainer(ctx context.Context, podName, containerName string, command []string) ([]byte, error) {
	k.debugLog("Exec in container:", podName, containerName, "command:", command)

	// Create exec request
	req := k.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(k.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   command,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	// Create SPDY executor
	exec, err := remotecommand.NewSPDYExecutor(k.restConfig, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	// Create buffers to capture output
	var stdout, stderr strings.Builder

	// Execute the command
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		k.debugLog("Exec failed, stderr:", stderr.String())
		return nil, fmt.Errorf("exec command failed: %w (stderr: %s)", err, stderr.String())
	}

	result := stdout.String()
	k.debugLog("Exec success, output length:", len(result))

	return []byte(result), nil
}

// createSidecarPodManifest creates a Pod manifest with main container + sidecar for log access
func (k *KubernetesContainerRuntime) createSidecarPodManifest(podName string, spec ContainerSpec, wrappedCommand string) *corev1.Pod {
	// Start with default labels and merge with spec labels for session tracking
	labels := map[string]string{
		"app":     "mcp-tool", // Different from mcp-server for POCI tools
		"tool":    sanitizeName(spec.Name),
		"runtime": "kubernetes",
		"type":    "poci-sidecar", // Mark as POCI tool with sidecar approach
	}

	// Merge session tracking labels from spec
	for key, value := range spec.Labels {
		labels[key] = value
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: k.namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever, // One-shot execution
			TerminationGracePeriodSeconds: &[]int64{5}[0],            // Fast cleanup

			// Shared volume for log files
			Volumes: []corev1.Volume{
				{
					Name: "logs",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},

			Containers: []corev1.Container{
				// Main container with ENTRYPOINT override
				{
					Name:  "main",
					Image: spec.Image,

					// Override ENTRYPOINT with shell wrapper
					Command: []string{"sh", "-c"},
					Args:    []string{wrappedCommand},

					// Mount logs volume
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "logs",
							MountPath: "/logs",
						},
					},
				},

				// Sidecar container for log access
				{
					Name:  "sidecar",
					Image: "alpine:3.22.1", // Will be pulled during gateway startup

					// Just sleep to stay alive
					Command: []string{"sleep", "3600"},

					// Mount same logs volume
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "logs",
							MountPath: "/logs",
						},
					},
				},
			},
		},
	}

	// Add environment variables to main container
	if len(spec.Env) > 0 {
		for key, value := range spec.Env {
			pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
				Name:  key,
				Value: value,
			})
		}
	}

	// Add environment variables from Kubernetes Secrets (secretKeyRef)
	if len(spec.SecretKeyRefs) > 0 {
		for envName, secretRef := range spec.SecretKeyRefs {
			pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
				Name: envName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: secretRef.Key,
					},
				},
			})
		}
	}

	// Add environment variables from Kubernetes ConfigMaps (envFrom)
	if len(spec.ConfigMapRefs) > 0 {
		for _, configMapName := range spec.ConfigMapRefs {
			pod.Spec.Containers[0].EnvFrom = append(pod.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			})
		}
	}

	return pod
}

// StopContainer stops a persistent Kubernetes Pod and cleans up resources
func (k *KubernetesContainerRuntime) StopContainer(ctx context.Context, handle *ContainerHandle) error {
	k.debugLog("StopContainer called for container:", handle.ID)

	// Close stdin/stdout streams if they exist
	if handle.Stdin != nil {
		if closer, ok := handle.Stdin.(io.Closer); ok {
			_ = closer.Close()
		}
	}
	if handle.Stdout != nil {
		if closer, ok := handle.Stdout.(io.Closer); ok {
			_ = closer.Close()
		}
	}

	// Delete the Pod using client-go with immediate termination
	deletePolicy := metav1.DeletePropagationForeground
	if err := k.clientset.CoreV1().Pods(k.namespace).Delete(ctx, handle.ID, metav1.DeleteOptions{
		PropagationPolicy:  &deletePolicy,
		GracePeriodSeconds: &[]int64{0}[0], // Immediate termination to prevent hanging
	}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete Pod: %w", err)
		}
		// Pod already deleted, which is fine
		k.debugLog("Pod already deleted:", handle.ID)
	} else {
		k.debugLog("Pod deleted successfully:", handle.ID)
	}

	return nil
}

// createPodManifest creates a Kubernetes Pod manifest from ContainerSpec
func (k *KubernetesContainerRuntime) createPodManifest(podName string, spec ContainerSpec) *corev1.Pod {
	// Start with default labels and merge with spec labels for session tracking
	labels := map[string]string{
		"app":     "mcp-server",
		"server":  sanitizeName(spec.Name),
		"runtime": "kubernetes",
	}

	// Merge session tracking labels from spec
	for key, value := range spec.Labels {
		labels[key] = value
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: k.namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever, // MCP servers are managed by the gateway
			TerminationGracePeriodSeconds: &[]int64{5}[0],            // Force terminate after 5 seconds to prevent hanging
			Containers: []corev1.Container{
				{
					Name:  "mcp-server",
					Image: spec.Image,
					// Enable stdio for MCP communication (equivalent to docker run -i)
					Stdin:     true,  // Required for Pod attach to work
					StdinOnce: false, // Keep stdin open for persistent MCP communication
					TTY:       false, // Usually false for JSON-based MCP protocol

					// Lifecycle hook to keep container alive after main process completes
					// This gives attach streams time to read stdout/stderr before container terminates
					Lifecycle: &corev1.Lifecycle{
						PreStop: &corev1.LifecycleHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"sleep", "10"}, // Keep alive 10s after main process exits
							},
						},
					},
				},
			},
		},
	}

	// Add command if specified
	// Use Args instead of Command to preserve the image's ENTRYPOINT
	// In Kubernetes: Command overrides ENTRYPOINT, Args appends to ENTRYPOINT
	if len(spec.Command) > 0 {
		pod.Spec.Containers[0].Args = spec.Command
	}

	// Add regular environment variables
	if len(spec.Env) > 0 {
		for key, value := range spec.Env {
			pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
				Name:  key,
				Value: value,
			})
		}
	}

	// Add environment variables from Kubernetes Secrets (secretKeyRef)
	if len(spec.SecretKeyRefs) > 0 {
		for envName, secretRef := range spec.SecretKeyRefs {
			pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
				Name: envName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: secretRef.Key,
					},
				},
			})
		}
	}

	// Add environment variables from Kubernetes ConfigMaps (envFrom)
	if len(spec.ConfigMapRefs) > 0 {
		for _, configMapName := range spec.ConfigMapRefs {
			pod.Spec.Containers[0].EnvFrom = append(pod.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			})
		}
	}

	return pod
}

// waitForPodReady waits for a Pod to reach Ready status
func (k *KubernetesContainerRuntime) waitForPodReady(ctx context.Context, podName string, timeoutSeconds int) error {
	k.debugLog("Waiting for Pod to be ready:", podName)

	// Use configurable timeout or default to 60 seconds
	timeout := 60 * time.Second
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}

	// TODO: Use proper watch API for better performance
	// For now, use polling with timeout
	interval := 2 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pod, err := k.clientset.CoreV1().Pods(k.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get Pod status: %w", err)
		}

		// Check if Pod is ready
		if isPodReady(pod) {
			return nil
		}

		// Check if Pod failed
		if pod.Status.Phase == corev1.PodFailed {
			// Get detailed failure information
			errorMsg := k.getPodFailureDetails(ctx, pod)
			return fmt.Errorf("pod failed: %s", errorMsg)
		}

		time.Sleep(interval)
	}

	return fmt.Errorf("timeout waiting for Pod to be ready")
}

// waitForPodCompletion waits for a Pod to complete and returns its exit code
func (k *KubernetesContainerRuntime) waitForPodCompletion(ctx context.Context, podName string) (int, error) {
	k.debugLog("waitForPodCompletion ENTRY for Pod:", podName)

	// Use a reasonable timeout for POCI tools (they should complete quickly)
	timeout := 120 * time.Second
	interval := 2 * time.Second
	deadline := time.Now().Add(timeout)
	k.debugLog("Using timeout:", timeout, "interval:", interval)

	for time.Now().Before(deadline) {
		k.debugLog("Polling Pod status...")
		pod, err := k.clientset.CoreV1().Pods(k.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			k.debugLog("ERROR: Failed to get Pod status:", err)
			return -1, fmt.Errorf("failed to get Pod status: %w", err)
		}

		k.debugLog("Pod phase:", pod.Status.Phase, "Container statuses:", len(pod.Status.ContainerStatuses))

		// With sidecar approach, check specifically for main container completion
		// (Pod will never reach Succeeded because sidecar keeps running)
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Name == "main" { // Our main container
				k.debugLog("Main container status:", containerStatus.Name, "State:", containerStatus.State)

				if containerStatus.State.Terminated != nil {
					// Main container has completed
					exitCode := int(containerStatus.State.Terminated.ExitCode)
					k.debugLog("Main container terminated with exit code:", exitCode)
					return exitCode, nil
				}

				if containerStatus.State.Waiting != nil {
					k.debugLog("Main container still waiting:", containerStatus.State.Waiting.Reason)
					break // Continue polling
				}

				if containerStatus.State.Running != nil {
					k.debugLog("Main container still running")
					break // Continue polling
				}
			}
		}

		k.debugLog("Pod still running, sleeping for", interval)
		// Pod still running, wait a bit more
		select {
		case <-ctx.Done():
			k.debugLog("Context cancelled while waiting for Pod completion")
			return -1, ctx.Err()
		case <-time.After(interval):
			// Continue polling
		}
	}

	return -1, fmt.Errorf("timeout waiting for Pod to complete")
}

// waitForCompletionMarker waits for the completion marker file to be created
func (k *KubernetesContainerRuntime) waitForCompletionMarker(ctx context.Context, podName string) error {
	k.debugLog("waitForCompletionMarker ENTRY for Pod:", podName)

	// Short timeout for completion marker (should appear quickly after main container terminates)
	timeout := 30 * time.Second
	interval := 1 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check if completion marker file exists
		_, err := k.execInContainer(ctx, podName, "sidecar", []string{"test", "-f", "/logs/complete.marker"})
		if err == nil {
			k.debugLog("Completion marker found!")
			return nil
		}

		k.debugLog("Completion marker not found yet, waiting...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
			// Continue checking
		}
	}

	return fmt.Errorf("timeout waiting for completion marker")
}

// getPodFailureDetails returns detailed information about why a Pod failed
func (k *KubernetesContainerRuntime) getPodFailureDetails(ctx context.Context, pod *corev1.Pod) string {
	var details []string

	// Add basic status information
	if pod.Status.Message != "" {
		details = append(details, fmt.Sprintf("Status: %s", pod.Status.Message))
	}
	if pod.Status.Reason != "" {
		details = append(details, fmt.Sprintf("Reason: %s", pod.Status.Reason))
	}

	// Check container statuses for detailed errors
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil {
			waiting := containerStatus.State.Waiting
			details = append(details, fmt.Sprintf("Container %s waiting: %s - %s",
				containerStatus.Name, waiting.Reason, waiting.Message))
		}
		if containerStatus.State.Terminated != nil {
			terminated := containerStatus.State.Terminated
			details = append(details, fmt.Sprintf("Container %s terminated: %s - %s (exit code: %d)",
				containerStatus.Name, terminated.Reason, terminated.Message, terminated.ExitCode))
		}
	}

	// Get Pod events for additional context
	events, err := k.clientset.CoreV1().Events(k.namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod.Name),
	})
	if err == nil && len(events.Items) > 0 {
		details = append(details, "Recent events:")
		for i, event := range events.Items {
			if i >= 5 { // Limit to last 5 events
				break
			}
			details = append(details, fmt.Sprintf("  - %s: %s", event.Reason, event.Message))
		}
	}

	// Get container logs if available
	if len(pod.Status.ContainerStatuses) > 0 {
		containerName := pod.Status.ContainerStatuses[0].Name
		if logs, err := k.getContainerLogs(ctx, pod.Name, containerName); err == nil && logs != "" {
			// Limit log output to last few lines
			logLines := strings.Split(strings.TrimSpace(logs), "\n")
			maxLines := 5
			if len(logLines) > maxLines {
				logLines = logLines[len(logLines)-maxLines:]
			}
			details = append(details, "Container logs:")
			for _, line := range logLines {
				details = append(details, fmt.Sprintf("  > %s", line))
			}
		}
	}

	if len(details) == 0 {
		return "Unknown failure"
	}

	return strings.Join(details, "\n")
}

// getContainerLogs retrieves recent logs from a container
func (k *KubernetesContainerRuntime) getContainerLogs(ctx context.Context, podName, containerName string) (string, error) {
	// Get last 10 lines of logs
	tailLines := int64(10)
	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
	}

	req := k.clientset.CoreV1().Pods(k.namespace).GetLogs(podName, logOptions)
	logs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer logs.Close()

	// Read log content
	logContent, err := io.ReadAll(logs)
	if err != nil {
		return "", err
	}

	return string(logContent), nil
}

// isPodReady checks if a Pod is in Ready condition
func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// createPodAttachStreams creates real stdio streams using Kubernetes Pod attach API
func (k *KubernetesContainerRuntime) createPodAttachStreams(ctx context.Context, podName string) (io.WriteCloser, io.ReadCloser, func() error, error) {
	k.debugLog("Creating Pod attach streams for:", podName)

	// Create attach request to Pod's main container process (not exec)
	req := k.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(k.namespace).
		SubResource("attach"). // Critical: use "attach" not "exec" for main process stdio
		Param("container", "mcp-server").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "false"). // MCP protocol typically doesn't need stderr
		Param("tty", "false")     // JSON-based MCP protocol doesn't need TTY

	k.debugLog("Pod attach URL:", req.URL())

	// Create SPDY executor for Pod attach (equivalent to kubectl attach)
	executor, err := remotecommand.NewSPDYExecutor(k.restConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	// Create pipes for bidirectional communication
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	// Start the attach stream in a goroutine
	streamErrCh := make(chan error, 1)
	go func() {
		streamOptions := remotecommand.StreamOptions{
			Stdin:  stdinReader,  // Read from our pipe, write to Pod stdin
			Stdout: stdoutWriter, // Read from Pod stdout, write to our pipe
			Stderr: nil,          // Not needed for MCP protocol
			Tty:    false,        // No TTY for JSON protocol
		}

		k.debugLog("Starting Pod attach stream for:", podName)
		err := executor.StreamWithContext(ctx, streamOptions)
		if err != nil {
			k.debugLog("Pod attach stream error:", err)
		}
		streamErrCh <- err

		// Close the pipes when stream ends
		stdinReader.Close()
		stdoutWriter.Close()
	}()

	// Create cleanup function
	cleanup := func() error {
		k.debugLog("Cleaning up Pod attach streams for:", podName)

		// Close our end of the pipes
		stdinWriter.Close()
		stdoutReader.Close()

		// Wait for stream to end (with timeout)
		select {
		case err := <-streamErrCh:
			if err != nil {
				k.debugLog("Pod attach stream ended with error:", err)
			}
			return err
		case <-time.After(5 * time.Second):
			k.debugLog("Pod attach stream cleanup timeout")
			return fmt.Errorf("timeout waiting for Pod attach stream to close")
		}
	}

	return stdinWriter, stdoutReader, cleanup, nil
}

// sanitizeName converts a name to be Kubernetes-compatible
func sanitizeName(name string) string {
	// Replace invalid characters with hyphens and convert to lowercase
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, "_", "-")
	result = strings.ReplaceAll(result, ".", "-")
	// TODO: Add more comprehensive sanitization if needed
	return result
}

// inspectImage retrieves ENTRYPOINT and CMD from a container image using registry APIs
func (k *KubernetesContainerRuntime) inspectImage(ctx context.Context, imageRef string) (entrypoint []string, cmd []string, err error) {
	fmt.Printf("[KubernetesRuntime] inspectImage: Starting inspection of %s\n", imageRef)
	k.debugLog("Inspecting image:", imageRef)

	// Parse image reference
	fmt.Printf("[KubernetesRuntime] inspectImage: Parsing image reference...\n")
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		fmt.Printf("[KubernetesRuntime] inspectImage: ERROR parsing reference: %v\n", err)
		return nil, nil, fmt.Errorf("failed to parse image reference: %w", err)
	}

	fmt.Printf("[KubernetesRuntime] inspectImage: Parsed reference: %s\n", ref.String())
	k.debugLog("Parsed image reference:", ref.String())

	// Pull image config from registry
	fmt.Printf("[KubernetesRuntime] inspectImage: Fetching image config from registry...\n")
	img, err := remote.Image(ref, remote.WithContext(ctx))
	if err != nil {
		fmt.Printf("[KubernetesRuntime] inspectImage: ERROR fetching from registry: %v\n", err)
		return nil, nil, fmt.Errorf("failed to fetch image from registry: %w", err)
	}

	fmt.Printf("[KubernetesRuntime] inspectImage: Successfully fetched image from registry\n")
	k.debugLog("Successfully fetched image from registry")

	// Get config file
	fmt.Printf("[KubernetesRuntime] inspectImage: Getting config file...\n")
	config, err := img.ConfigFile()
	if err != nil {
		fmt.Printf("[KubernetesRuntime] inspectImage: ERROR getting config file: %v\n", err)
		return nil, nil, fmt.Errorf("failed to get image config: %w", err)
	}

	fmt.Printf("[KubernetesRuntime] inspectImage: Successfully retrieved image config\n")
	k.debugLog("Successfully retrieved image config")

	// Extract ENTRYPOINT and CMD
	entrypoint = config.Config.Entrypoint
	cmd = config.Config.Cmd

	fmt.Printf("[KubernetesRuntime] inspectImage: Extracted ENTRYPOINT=%v CMD=%v\n", entrypoint, cmd)
	k.debugLog("Image config inspection complete:")
	k.debugLog("  Raw ENTRYPOINT:", entrypoint)
	k.debugLog("  Raw CMD:", cmd)

	return entrypoint, cmd, nil
}

// debugLog prints debug messages if verbose is enabled
func (k *KubernetesContainerRuntime) debugLog(args ...any) {
	if k.verbose {
		prefixedArgs := append([]any{"[KubernetesContainerRuntime]"}, args...)
		fmt.Fprintln(os.Stderr, prefixedArgs...)
	}
}
