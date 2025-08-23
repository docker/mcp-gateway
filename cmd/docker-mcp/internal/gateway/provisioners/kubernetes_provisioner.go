package provisioners

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/runtime"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/secrets"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

// KubernetesProvisionerImpl implements the Provisioner interface for Kubernetes Pod deployment
type KubernetesProvisionerImpl struct {
	containerRuntime        runtime.ContainerRuntime
	configResolver          ConfigResolver                   // Just-in-time config resolution
	secretManager           KubernetesSecretManager          // Kubernetes Secret management
	configManager           KubernetesConfigManager          // Kubernetes ConfigMap management
	serverConfigs           map[string]*catalog.ServerConfig // Server configurations for cluster secret generation
	namespace               string
	secretName              string             // Gateway-level secret name (e.g., "mcp-gateway-secrets")
	secretProvider          SecretProviderType // Secret provider strategy
	configProvider          ConfigProviderType // Configuration provider strategy
	configName              string             // ConfigMap resource name (e.g., "mcp-gateway-config")
	maxServerStartupTimeout int                // Maximum time in seconds to wait for each server to start
	verbose                 bool
	sessionID               string // Session ID for resource tracking and cleanup
	capRefresher            mcpclient.CapabilityRefresher
}

// KubernetesProvisionerConfig holds configuration for creating a Kubernetes provisioner
type KubernetesProvisionerConfig struct {
	ContainerRuntime        runtime.ContainerRuntime
	ConfigResolver          ConfigResolver          // Just-in-time config resolution
	SecretManager           KubernetesSecretManager // Kubernetes Secret management
	Namespace               string
	SecretName              string             // Gateway-level secret name (e.g., "mcp-gateway-secrets")
	SecretProvider          SecretProviderType // Secret provider strategy
	ConfigProvider          ConfigProviderType // Configuration provider strategy
	ConfigName              string             // ConfigMap resource name (e.g., "mcp-gateway-config")
	MaxServerStartupTimeout int                // Maximum time in seconds to wait for each server to start
	Verbose                 bool
	SessionID               string // Session ID for resource tracking and cleanup
	CapRefresher            mcpclient.CapabilityRefresher
}

// NewKubernetesProvisioner creates a new Kubernetes provisioner instance
func NewKubernetesProvisioner(config KubernetesProvisionerConfig) *KubernetesProvisionerImpl {
	// Use default namespace if not specified
	namespace := config.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Use default secret name if not specified
	secretName := config.SecretName
	if secretName == "" {
		secretName = "mcp-gateway-secrets"
	}

	// Use default config name if not specified
	configName := config.ConfigName
	if configName == "" {
		configName = "mcp-gateway-config"
	}

	return &KubernetesProvisionerImpl{
		containerRuntime:        config.ContainerRuntime,
		configResolver:          config.ConfigResolver,
		secretManager:           config.SecretManager,
		namespace:               namespace,
		secretName:              secretName,
		secretProvider:          config.SecretProvider,
		configProvider:          config.ConfigProvider,
		configName:              configName,
		maxServerStartupTimeout: config.MaxServerStartupTimeout,
		verbose:                 config.Verbose,
		sessionID:               config.SessionID,
		capRefresher:            config.CapRefresher,
	}
}

// GetName returns the unique identifier for this provisioner type
func (kp *KubernetesProvisionerImpl) GetName() string {
	return KubernetesProvisioner.String()
}

// Initialize sets up the provisioner with configuration and dependencies
func (kp *KubernetesProvisionerImpl) Initialize(ctx context.Context, configResolver ConfigResolver, serverConfigs map[string]*catalog.ServerConfig) error {
	kp.debugLog("Initialize called")

	// Set up the config resolver and server configs
	kp.configResolver = configResolver
	kp.serverConfigs = serverConfigs
	kp.debugLog("ConfigResolver and ServerConfigs injected")

	// Create and set up the secret manager based on secret provider strategy
	if configResolver != nil {
		switch kp.secretProvider {
		case DockerEngineSecretProvider:
			// Use gateway secret manager for just-in-time secret provisioning
			secretManager := NewGatewayKubernetesSecretManager(configResolver, serverConfigs, kp.secretName)
			kp.secretManager = secretManager
			kp.debugLog("GatewayKubernetesSecretManager created for docker-engine provider with secret name:", kp.secretName)
		case ClusterSecretProvider:
			// No secret manager needed for cluster mode - secrets are pre-existing
			kp.secretManager = nil
			kp.debugLog("No secret manager created for cluster provider - using pre-existing secrets with name:", kp.secretName)
		default:
			kp.debugLog("Warning: Unknown secret provider, defaulting to docker-engine")
			secretManager := NewGatewayKubernetesSecretManager(configResolver, serverConfigs, kp.secretName)
			kp.secretManager = secretManager
		}

		// Create and set up the config manager based on config provider strategy
		switch kp.configProvider {
		case DockerEngineConfigProvider:
			// Use gateway config manager for just-in-time ConfigMap provisioning
			configManager := NewGatewayKubernetesConfigManager(configResolver, serverConfigs, kp.configName)
			kp.configManager = configManager
			kp.debugLog("GatewayKubernetesConfigManager created for docker-engine provider with ConfigMap name:", kp.configName)
		case ClusterConfigProvider:
			// No config manager needed for cluster mode - ConfigMaps are pre-existing
			kp.configManager = nil
			kp.debugLog("No config manager created for cluster provider - using pre-existing ConfigMaps with name:", kp.configName)
		default:
			kp.debugLog("Warning: Unknown config provider, defaulting to docker-engine")
			configManager := NewGatewayKubernetesConfigManager(configResolver, serverConfigs, kp.configName)
			kp.configManager = configManager
		}
	}

	// Create shared resources once during initialization to avoid race conditions
	if err := kp.createSharedResources(ctx); err != nil {
		kp.debugLog("Warning: Failed to create shared resources during initialization:", err)
		// Continue - this is not fatal, resources will be created on-demand
	}

	return nil
}

// SetConfigResolver injects a config resolver for just-in-time secret/config resolution
// Deprecated: Use Initialize method instead
func (kp *KubernetesProvisionerImpl) SetConfigResolver(resolver ConfigResolver) {
	kp.configResolver = resolver
	kp.debugLog("ConfigResolver injected")
}

// PreValidateDeployment checks if the given spec can be provisioned without actually deploying
func (kp *KubernetesProvisionerImpl) PreValidateDeployment(_ context.Context, spec ProvisionerSpec) error {
	kp.debugLog("PreValidateDeployment called for server:", spec.Name)

	// Basic validation
	if spec.Name == "" {
		kp.debugLog("Validation failed: server name is required")
		return fmt.Errorf("server name is required")
	}

	// For Kubernetes provisioner, we need an image for containerized deployment
	if spec.Image == "" {
		kp.debugLog("Validation failed for server:", spec.Name, "- container image is required for Kubernetes deployment")
		return fmt.Errorf("container image is required for Kubernetes deployment")
	}

	// TODO: Add more sophisticated validation:
	// - Check kubectl connectivity to cluster
	// - Validate namespace exists or can be created
	// - Check RBAC permissions for required operations
	// - Validate image registry access
	// - Check resource quotas

	kp.debugLog("PreValidateDeployment passed for server:", spec.Name)
	return nil
}

// ProvisionServer deploys the server according to the spec and returns a client handle
func (kp *KubernetesProvisionerImpl) ProvisionServer(ctx context.Context, spec ProvisionerSpec, ss *mcp.ServerSession, server *mcp.Server) (mcpclient.Client, func(), error) {
	kp.debugLog("ProvisionServer called for server:", spec.Name)
	cleanup := func() {}
	var client mcpclient.Client

	// Convert spec back to serverConfig for compatibility with existing logic
	// This is temporary until we fully migrate to the provisioner interface
	serverConfig := kp.specToServerConfig(spec)
	kp.debugLog("Converted spec to server config for:", spec.Name)

	// Decision tree for client type (similar to Docker provisioner pattern)
	if serverConfig.Spec.Remote.URL != "" {
		// Remote HTTP endpoint (reuse existing remote client)
		kp.debugLog("Using remote HTTP endpoint for:", spec.Name, "URL:", serverConfig.Spec.Remote.URL)
		client = mcpclient.NewRemoteMCPClient(serverConfig)
	} else if spec.LongLived {
		// Long-lived containerized stdio (Kubernetes Pod) - use Container Runtime for persistent containers
		kp.debugLog("Using persistent containerized stdio via Kubernetes Container Runtime for long-lived MCP server:", spec.Name)

		// Note: Shared Kubernetes resources (Secrets/ConfigMaps) are created during provisioner initialization
		// to avoid race conditions during concurrent server provisioning

		// Convert ProvisionerSpec to ContainerSpec for Container Runtime
		containerSpec := kp.buildContainerSpec(spec)
		containerSpec.Persistent = true // Ensure persistent for long-lived

		kp.debugLog("Starting persistent Kubernetes Pod", containerSpec.Image, "via Container Runtime")

		// Start persistent container using Kubernetes Container Runtime
		handle, err := kp.containerRuntime.StartContainer(ctx, containerSpec)
		if err != nil {
			kp.debugLog("Failed to start Kubernetes Pod for:", spec.Name, "error:", err)
			return nil, cleanup, fmt.Errorf("failed to start Kubernetes Pod: %w", err)
		}

		// Create MCP client using Container Runtime handles
		client = mcpclient.NewStdioHandleClient(serverConfig.Name, handle.Stdin, handle.Stdout)

		// Update cleanup to stop the Pod
		originalCleanup := cleanup
		cleanup = func() {
			// Stop Pod first
			ctx := context.Background() // Use background context for cleanup
			if stopErr := kp.containerRuntime.StopContainer(ctx, handle); stopErr != nil {
				kp.debugLog("Error stopping Kubernetes Pod for:", spec.Name, "error:", stopErr)
			}
			// Then run original cleanup
			originalCleanup()
		}
	} else {
		// Short-lived containerized stdio (Kubernetes Pod) - use Container Runtime but with ephemeral behavior
		kp.debugLog("Using ephemeral containerized stdio via Kubernetes Container Runtime for short-lived MCP server:", spec.Name)

		// Note: Shared Kubernetes resources (Secrets/ConfigMaps) are created during provisioner initialization
		// to avoid race conditions during concurrent server provisioning

		// Convert ProvisionerSpec to ContainerSpec for Container Runtime
		containerSpec := kp.buildContainerSpec(spec)
		containerSpec.Persistent = false // Ephemeral behavior

		kp.debugLog("Starting ephemeral Kubernetes Pod", containerSpec.Image, "via Container Runtime")

		// Start ephemeral container using Kubernetes Container Runtime
		handle, err := kp.containerRuntime.StartContainer(ctx, containerSpec)
		if err != nil {
			kp.debugLog("Failed to start ephemeral Kubernetes Pod for:", spec.Name, "error:", err)
			return nil, cleanup, fmt.Errorf("failed to start ephemeral Kubernetes Pod: %w", err)
		}

		// Create MCP client using Container Runtime handles
		client = mcpclient.NewStdioHandleClient(serverConfig.Name, handle.Stdin, handle.Stdout)

		// For ephemeral servers, cleanup immediately when client closes
		originalCleanup := cleanup
		cleanup = func() {
			// Stop Pod immediately for ephemeral behavior
			ctx := context.Background() // Use background context for cleanup
			if stopErr := kp.containerRuntime.StopContainer(ctx, handle); stopErr != nil {
				kp.debugLog("Error stopping ephemeral Kubernetes Pod for:", spec.Name, "error:", stopErr)
			}
			// Then run original cleanup
			originalCleanup()
		}
	}

	// Initialize the client (same as Docker provisioner)
	initParams := &mcp.InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: &mcp.Implementation{
			Name:    "kubernetes",
			Version: "1.0.0",
		},
	}

	// Initialize the client
	kp.debugLog("Initializing MCP client for:", spec.Name)
	if err := client.Initialize(ctx, initParams, kp.verbose, ss, server, kp.capRefresher); err != nil {
		kp.debugLog("Failed to initialize MCP client for:", spec.Name, "error:", err)
		return nil, cleanup, err
	}

	kp.debugLog("Successfully provisioned Kubernetes server:", spec.Name)
	return client, cleanup, nil
}

// Shutdown performs cleanup of all resources managed by this provisioner
func (kp *KubernetesProvisionerImpl) Shutdown(ctx context.Context) error {
	kp.debugLog("Shutting down Kubernetes provisioner")

	// Perform session-based cleanup if session ID is available
	if err := kp.CleanupSession(ctx); err != nil {
		// Check if this is a context cancellation error during shutdown
		if ctx.Err() == context.Canceled {
			kp.debugLog("Session cleanup canceled due to shutdown signal")
		} else {
			kp.debugLog("Error during session cleanup:", err)
		}
		// Don't fail shutdown for cleanup errors, just log them
	}

	// Let the container runtime perform its own shutdown
	if err := kp.containerRuntime.Shutdown(ctx); err != nil {
		kp.debugLog("Error during container runtime shutdown:", err)
		// Don't fail shutdown for cleanup errors, just log them
	}

	kp.debugLog("Kubernetes provisioner shutdown completed")
	return nil
}

// specToServerConfig creates a minimal catalog.ServerConfig for compatibility
// Note: This method doesn't include secrets since they're resolved just-in-time
func (kp *KubernetesProvisionerImpl) specToServerConfig(spec ProvisionerSpec) *catalog.ServerConfig {
	// Convert environment map to catalog.Env slice
	var env []catalog.Env
	for name, value := range spec.Environment {
		env = append(env, catalog.Env{Name: name, Value: value})
	}

	return &catalog.ServerConfig{
		Name: spec.Name,
		Spec: catalog.Server{
			Image:          spec.Image,
			Command:        spec.Command,
			Env:            env,
			Secrets:        []catalog.Secret{}, // Empty - secrets resolved via ConfigResolver
			Volumes:        spec.Volumes,
			DisableNetwork: spec.DisableNetwork,
			LongLived:      spec.LongLived,
		},
		Config:  map[string]any{},    // Empty for now
		Secrets: map[string]string{}, // Empty - secrets resolved via ConfigResolver
	}
}

// buildContainerSpec converts a ProvisionerSpec to a ContainerSpec for Container Runtime
func (kp *KubernetesProvisionerImpl) buildContainerSpec(spec ProvisionerSpec) runtime.ContainerSpec {
	// Start with non-sensitive environment variables from spec
	env := make(map[string]string)
	for name, value := range spec.Environment {
		env[name] = value
	}

	// For Kubernetes: Generate secretKeyRef mappings based on secret provider strategy
	var secretKeyRefs map[string]runtime.SecretKeyRef
	if kp.secretManager != nil {
		// Docker-engine mode: Use SecretManager to get secretKeyRef mappings
		secretKeySelectors := kp.secretManager.GetSecretKeyRefs(spec.Name)
		secretKeyRefs = make(map[string]runtime.SecretKeyRef)
		for envName, selector := range secretKeySelectors {
			secretKeyRefs[envName] = runtime.SecretKeyRef{
				Name: selector.Name,
				Key:  selector.Key,
			}
		}
	} else if kp.secretProvider == ClusterSecretProvider {
		// Cluster mode: Generate secretKeyRefs directly for pre-existing secrets
		secretKeyRefs = kp.generateClusterSecretKeyRefs(spec.Name)
	}

	// Resolve non-secret environment variables based on configuration provider strategy
	if kp.configProvider == DockerEngineConfigProvider && kp.configResolver != nil {
		// Docker-engine mode: Use ConfigResolver for just-in-time template resolution
		resolvedEnv := kp.configResolver.ResolveEnvironment(spec.Name)
		for envName, envValue := range resolvedEnv {
			env[envName] = envValue
		}
	} else if kp.configProvider == ClusterConfigProvider {
		// Cluster mode: Environment variables will be injected from ConfigMap
		kp.debugLog("Using cluster mode for environment variables - will be injected from ConfigMap:", kp.configName)
		// Note: In cluster mode, environment variables are handled via envFrom in the Pod spec
		// They are not added to the env map here since they come from ConfigMap at runtime
	}

	// Just-in-time command resolution (for template substitution)
	command := spec.Command
	if kp.configResolver != nil {
		command = kp.configResolver.ResolveCommand(spec.Name)
	}

	// Add session labels for resource tracking and cleanup
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "mcp-gateway",
		"app.kubernetes.io/component":  "mcp-server",
		"app.kubernetes.io/name":       spec.Name,
	}
	if kp.sessionID != "" {
		labels["app.kubernetes.io/instance"] = kp.sessionID
		labels["mcp-gateway.docker.com/session"] = kp.sessionID
	}

	// Configure ConfigMap references based on configuration provider strategy
	var configMapRefs []string
	if kp.configProvider == ClusterConfigProvider {
		// Cluster mode: Reference pre-existing ConfigMaps
		configMapRefs = []string{kp.configName}
	} else if kp.configProvider == DockerEngineConfigProvider && kp.configManager != nil {
		// Docker-engine mode: Use configManager to get ConfigMap references for just-in-time created ConfigMaps
		configMapRefs = kp.configManager.GetConfigMapRefs(spec.Name)
	}

	return runtime.ContainerSpec{
		Name:    spec.Name,
		Image:   spec.Image,
		Command: command,

		// Kubernetes-specific runtime configuration
		// Note: Networks field is not used for Kubernetes - networking handled by Pod spec
		Networks: []string{}, // Empty for Kubernetes
		Volumes:  spec.Volumes,
		Env:      env,
		Labels:   labels, // Session tracking labels for cleanup

		// Kubernetes Secret references for environment variables
		SecretKeyRefs: secretKeyRefs,

		// Kubernetes ConfigMap references for environment variables
		ConfigMapRefs: configMapRefs,

		// Resource limits (will be converted to Kubernetes resource requests/limits)
		CPUs:   0,  // TODO: Extract from spec or provisioner config
		Memory: "", // TODO: Extract from spec or provisioner config

		// Container configuration for MCP servers (ephemeral vs persistent determined by caller)
		Persistent:     spec.LongLived,             // Use spec setting - long-lived servers are persistent
		AttachStdio:    true,                       // Need stdin/stdout for MCP protocol
		KeepStdinOpen:  true,                       // Keep stdin open for ongoing MCP communication
		RestartPolicy:  "no",                       // No auto-restart for MCP servers (handled by Kubernetes)
		StartupTimeout: kp.maxServerStartupTimeout, // Configurable server startup timeout

		// Security and behavior (Kubernetes-specific interpretations)
		RemoveAfterRun: false, // Ignored - Kubernetes Pods managed by lifecycle
		Interactive:    true,  // Required for MCP communication
		Init:           false, // Kubernetes handles init containers differently
		Privileged:     false, // Never privileged for MCP servers

		// Network isolation
		DisableNetwork: spec.DisableNetwork,
	}
}

// CleanupSession removes all pods associated with this provisioner's session ID
func (kp *KubernetesProvisionerImpl) CleanupSession(ctx context.Context) error {
	if kp.sessionID == "" {
		kp.debugLog("No session ID set, skipping cleanup")
		return nil
	}

	kp.debugLog("Starting session cleanup for session:", kp.sessionID)

	// Get Kubernetes client from container runtime
	k8sRuntime, ok := kp.containerRuntime.(*runtime.KubernetesContainerRuntime)
	if !ok {
		kp.debugLog("Error: container runtime is not KubernetesContainerRuntime")
		return fmt.Errorf("container runtime is not KubernetesContainerRuntime")
	}

	kp.debugLog("Getting Kubernetes clientset...")
	clientset, err := k8sRuntime.GetClientset()
	if err != nil {
		kp.debugLog("Error: failed to get Kubernetes clientset:", err)
		return fmt.Errorf("failed to get Kubernetes clientset: %w", err)
	}

	kp.debugLog("Successfully got Kubernetes clientset, proceeding with cleanup")

	// List pods with session label
	labelSelector := fmt.Sprintf("mcp-gateway.docker.com/session=%s", kp.sessionID)
	pods, err := clientset.CoreV1().Pods(kp.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods for session cleanup (check RBAC permissions for pods.list in namespace %s): %w", kp.namespace, err)
	}

	// Delete pods with foreground propagation
	deletePolicy := metav1.DeletePropagationForeground
	if len(pods.Items) == 0 {
		kp.debugLog("No pods found for session:", kp.sessionID)
	} else {
		kp.debugLog("Found", len(pods.Items), "pods to clean up for session:", kp.sessionID)
		kp.debugLog("About to start pod deletion loop for", len(pods.Items), "pods")
		for _, pod := range pods.Items {
			kp.debugLog("Deleting pod:", pod.Name)
			err := clientset.CoreV1().Pods(kp.namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			})
			if err != nil {
				kp.debugLog("Warning: Failed to delete pod (check RBAC permissions for pods.delete):", pod.Name, "error:", err)
			}
		}
		kp.debugLog("Pod deletion loop completed, moving to secret cleanup")
	}

	kp.debugLog("*** CRITICAL: Pod cleanup completed, starting secret cleanup phase ***")
	// Clean up Secrets only when using docker-engine provider (cluster provider doesn't create secrets)
	kp.debugLog("Secret provider mode:", kp.secretProvider, "comparing to DockerEngineSecretProvider:", DockerEngineSecretProvider)
	if kp.secretProvider == DockerEngineSecretProvider {
		kp.debugLog("Looking for secrets with label selector:", labelSelector)
		secrets, err := clientset.CoreV1().Secrets(kp.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			kp.debugLog("Warning: Failed to list secrets for session cleanup (check RBAC permissions for secrets.list in namespace", kp.namespace, "):", err)
		} else {
			kp.debugLog("Found", len(secrets.Items), "total secrets matching session label")
			if len(secrets.Items) > 0 {
				kp.debugLog("Found", len(secrets.Items), "secrets to clean up for session:", kp.sessionID)
				for _, secret := range secrets.Items {
					kp.debugLog("Deleting secret:", secret.Name)
					err := clientset.CoreV1().Secrets(kp.namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{
						PropagationPolicy: &deletePolicy,
					})
					if err != nil {
						kp.debugLog("Warning: Failed to delete secret (check RBAC permissions for secrets.delete):", secret.Name, "error:", err)
					} else {
						kp.debugLog("Successfully initiated deletion of secret:", secret.Name)

						// Wait for secret to be actually deleted to ensure cleanup completes
						kp.debugLog("Waiting for secret deletion to complete:", secret.Name)
						deleted := false
						for range 30 { // Wait up to 3 seconds
							_, err := clientset.CoreV1().Secrets(kp.namespace).Get(ctx, secret.Name, metav1.GetOptions{})
							if err != nil && strings.Contains(err.Error(), "not found") {
								kp.debugLog("Secret deletion confirmed:", secret.Name)
								deleted = true
								break
							}
							time.Sleep(100 * time.Millisecond)
						}
						if !deleted {
							kp.debugLog("Warning: Secret deletion timed out, but deletion was initiated:", secret.Name)
						}
					}
				}
			} else {
				kp.debugLog("No secrets found to clean up for session:", kp.sessionID)
			}
		}
	} else {
		kp.debugLog("Skipping secret cleanup for cluster provider (secrets are pre-existing)")
	}

	kp.debugLog("Session cleanup completed for session:", kp.sessionID)
	return nil
}

// CleanupStalePods removes pods older than the specified duration across all namespaces (if permitted)
func (kp *KubernetesProvisionerImpl) CleanupStalePods(ctx context.Context, maxAge time.Duration) error {
	kp.debugLog("Starting stale pod cleanup with max age:", maxAge)

	// Get Kubernetes client from container runtime
	k8sRuntime, ok := kp.containerRuntime.(*runtime.KubernetesContainerRuntime)
	if !ok {
		return fmt.Errorf("container runtime is not KubernetesContainerRuntime")
	}

	clientset, err := k8sRuntime.GetClientset()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes clientset: %w", err)
	}

	// List pods managed by mcp-gateway
	labelSelector := "app.kubernetes.io/managed-by=mcp-gateway"
	pods, err := clientset.CoreV1().Pods(kp.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list managed pods (check RBAC permissions for pods.list in namespace %s): %w", kp.namespace, err)
	}

	if len(pods.Items) == 0 {
		kp.debugLog("No managed pods found for stale cleanup")
		return nil
	}

	cutoffTime := time.Now().Add(-maxAge)
	var stalePods []corev1.Pod

	for _, pod := range pods.Items {
		if pod.CreationTimestamp.Time.Before(cutoffTime) {
			stalePods = append(stalePods, pod)
		}
	}

	if len(stalePods) == 0 {
		kp.debugLog("No stale pods found (older than", maxAge, ")")
		return nil
	}

	kp.debugLog("Found", len(stalePods), "stale pods to clean up")

	// Delete stale pods with foreground propagation
	deletePolicy := metav1.DeletePropagationForeground
	for _, pod := range stalePods {
		age := time.Since(pod.CreationTimestamp.Time)
		sessionID := pod.Labels["mcp-gateway.docker.com/session"]
		kp.debugLog("Deleting stale pod:", pod.Name, "age:", age, "session:", sessionID)

		err := clientset.CoreV1().Pods(kp.namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		})
		if err != nil {
			kp.debugLog("Warning: Failed to delete stale pod:", pod.Name, "error:", err)
		}
	}

	// Clean up stale Secrets only when using docker-engine provider (cluster provider doesn't create secrets)
	if kp.secretProvider == DockerEngineSecretProvider {
		secretLabelSelector := "app.kubernetes.io/managed-by=mcp-gateway,app.kubernetes.io/component=mcp-server-secret"
		secrets, err := clientset.CoreV1().Secrets(kp.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: secretLabelSelector,
		})
		if err != nil {
			kp.debugLog("Warning: Failed to list managed secrets for stale cleanup:", err)
		} else {
			var staleSecrets []corev1.Secret
			for _, secret := range secrets.Items {
				if secret.CreationTimestamp.Time.Before(cutoffTime) {
					staleSecrets = append(staleSecrets, secret)
				}
			}

			if len(staleSecrets) > 0 {
				kp.debugLog("Found", len(staleSecrets), "stale secrets to clean up")
				for _, secret := range staleSecrets {
					age := time.Since(secret.CreationTimestamp.Time)
					sessionID := secret.Labels["mcp-gateway.docker.com/session"]
					kp.debugLog("Deleting stale secret:", secret.Name, "age:", age, "session:", sessionID)

					err := clientset.CoreV1().Secrets(kp.namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{
						PropagationPolicy: &deletePolicy,
					})
					if err != nil {
						kp.debugLog("Warning: Failed to delete stale secret:", secret.Name, "error:", err)
					}
				}
			}
		}
	} else {
		kp.debugLog("Skipping stale secret cleanup for cluster provider (secrets are pre-existing)")
	}

	kp.debugLog("Stale resource cleanup completed")
	return nil
}

// createKubernetesSecrets creates Kubernetes Secret resources for the server
//
//nolint:unused // Reserved for future implementation
func (kp *KubernetesProvisionerImpl) createKubernetesSecrets(ctx context.Context, serverName string) error {
	if kp.secretManager == nil {
		kp.debugLog("No secret manager configured, skipping secret creation for:", serverName)
		return nil
	}

	// Get secret specifications from the secret manager
	secretSpecs := kp.secretManager.GetSecretSpecs(serverName)
	if len(secretSpecs) == 0 {
		kp.debugLog("No secrets to create for server:", serverName)
		return nil
	}

	// Get Kubernetes client from container runtime
	k8sRuntime, ok := kp.containerRuntime.(*runtime.KubernetesContainerRuntime)
	if !ok {
		return fmt.Errorf("container runtime is not KubernetesContainerRuntime")
	}

	clientset, err := k8sRuntime.GetClientset()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes clientset: %w", err)
	}

	// Create each secret resource
	for secretName, secretData := range secretSpecs {
		kp.debugLog("Creating Kubernetes Secret:", secretName, "for server:", serverName)

		// Add session labels for cleanup
		labels := map[string]string{
			"app.kubernetes.io/managed-by": "mcp-gateway",
			"app.kubernetes.io/component":  "mcp-server-secret",
			"app.kubernetes.io/name":       serverName,
		}
		if kp.sessionID != "" {
			labels["app.kubernetes.io/instance"] = kp.sessionID
			labels["mcp-gateway.docker.com/session"] = kp.sessionID
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: kp.namespace,
				Labels:    labels,
			},
			Type:       corev1.SecretTypeOpaque,
			StringData: secretData, // Use StringData for automatic base64 encoding
		}

		// Create or update the secret
		_, err := clientset.CoreV1().Secrets(kp.namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			// If secret already exists, try to update it
			if strings.Contains(err.Error(), "already exists") {
				kp.debugLog("Secret", secretName, "already exists, updating it")
				_, err = clientset.CoreV1().Secrets(kp.namespace).Update(ctx, secret, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("failed to update existing Kubernetes Secret %s: %w", secretName, err)
				}
			} else {
				return fmt.Errorf("failed to create Kubernetes Secret %s: %w", secretName, err)
			}
		}

		kp.debugLog("Successfully created/updated Kubernetes Secret:", secretName)
	}

	return nil
}

// createKubernetesConfigMaps creates Kubernetes ConfigMap resources for the server
//
//nolint:unused // Reserved for future implementation
func (kp *KubernetesProvisionerImpl) createKubernetesConfigMaps(ctx context.Context, serverName string) error {
	if kp.configManager == nil {
		kp.debugLog("No config manager configured, skipping ConfigMap creation for:", serverName)
		return nil
	}

	// Get ConfigMap specifications from the config manager
	configSpecs := kp.configManager.GetConfigSpecs(serverName)
	if len(configSpecs) == 0 {
		kp.debugLog("No ConfigMaps to create for server:", serverName)
		return nil
	}

	// Get Kubernetes client from container runtime
	k8sRuntime, ok := kp.containerRuntime.(*runtime.KubernetesContainerRuntime)
	if !ok {
		return fmt.Errorf("container runtime is not KubernetesContainerRuntime")
	}

	clientset, err := k8sRuntime.GetClientset()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes clientset: %w", err)
	}

	// Create each ConfigMap resource
	for configMapName, configData := range configSpecs {
		kp.debugLog("Creating Kubernetes ConfigMap:", configMapName, "for server:", serverName)

		// Add session labels for cleanup
		labels := map[string]string{
			"app.kubernetes.io/managed-by": "mcp-gateway",
			"app.kubernetes.io/component":  "mcp-server-config",
			"app.kubernetes.io/name":       serverName,
		}
		if kp.sessionID != "" {
			labels["app.kubernetes.io/instance"] = kp.sessionID
			labels["mcp-gateway.docker.com/session"] = kp.sessionID
		}

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: kp.namespace,
				Labels:    labels,
			},
			Data: configData,
		}

		// Create or update the ConfigMap
		_, err := clientset.CoreV1().ConfigMaps(kp.namespace).Create(ctx, configMap, metav1.CreateOptions{})
		if err != nil {
			// If ConfigMap already exists, try to update it
			if strings.Contains(err.Error(), "already exists") {
				kp.debugLog("ConfigMap", configMapName, "already exists, updating it")
				_, err = clientset.CoreV1().ConfigMaps(kp.namespace).Update(ctx, configMap, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("failed to update existing Kubernetes ConfigMap %s: %w", configMapName, err)
				}
			} else {
				return fmt.Errorf("failed to create Kubernetes ConfigMap %s: %w", configMapName, err)
			}
		}

		kp.debugLog("Successfully created/updated Kubernetes ConfigMap:", configMapName)
	}

	return nil
}

// generateClusterSecretKeyRefs generates secretKeyRef mappings for pre-existing cluster secrets
func (kp *KubernetesProvisionerImpl) generateClusterSecretKeyRefs(serverName string) map[string]runtime.SecretKeyRef {
	serverConfig, exists := kp.serverConfigs[serverName]
	if !exists {
		return map[string]runtime.SecretKeyRef{}
	}

	secretKeyRefs := make(map[string]runtime.SecretKeyRef)

	// Import the secrets package to use TemplateToSecretKey
	// Note: This import should already be available since we use it in other parts
	for _, secret := range serverConfig.Spec.Secrets {
		// Use consistent template mapping: secret.Name is the template like "dockerhub.username"
		secretKey := secrets.TemplateToSecretKey(secret.Name)

		secretKeyRefs[secret.Env] = runtime.SecretKeyRef{
			Name: kp.secretName, // Reference pre-existing secret
			Key:  secretKey,
		}
	}

	return secretKeyRefs
}

// createSharedResources creates shared Kubernetes resources once during initialization
func (kp *KubernetesProvisionerImpl) createSharedResources(ctx context.Context) error {
	kp.debugLog("Creating shared Kubernetes resources during initialization")

	// Create shared secrets if we have a secret manager (docker-engine mode)
	if kp.secretManager != nil {
		if err := kp.createSharedSecrets(ctx); err != nil {
			return fmt.Errorf("failed to create shared secrets: %w", err)
		}
	}

	// Create shared ConfigMaps if we have a config manager (docker-engine mode)
	if kp.configManager != nil {
		if err := kp.createSharedConfigMaps(ctx); err != nil {
			return fmt.Errorf("failed to create shared ConfigMaps: %w", err)
		}
	}

	kp.debugLog("Shared Kubernetes resources created successfully")
	return nil
}

// createSharedSecrets creates the shared Kubernetes Secret once with all server secrets
func (kp *KubernetesProvisionerImpl) createSharedSecrets(ctx context.Context) error {
	if kp.secretManager == nil {
		return nil // No secrets to create in cluster mode
	}

	secretSpecs := kp.secretManager.GetSecretSpecs("") // Empty server name gets all secrets
	if len(secretSpecs) == 0 {
		kp.debugLog("No secrets to create during initialization")
		return nil
	}

	// Get Kubernetes client from container runtime
	k8sRuntime, ok := kp.containerRuntime.(*runtime.KubernetesContainerRuntime)
	if !ok {
		return fmt.Errorf("container runtime is not KubernetesContainerRuntime")
	}

	clientset, err := k8sRuntime.GetClientset()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes clientset: %w", err)
	}

	for secretName, secretData := range secretSpecs {
		kp.debugLog("Creating shared Kubernetes Secret:", secretName)

		// Add session labels for cleanup
		labels := map[string]string{
			"app.kubernetes.io/managed-by": "mcp-gateway",
			"app.kubernetes.io/component":  "mcp-server-secret",
			"app.kubernetes.io/name":       "shared-secrets",
		}
		if kp.sessionID != "" {
			labels["app.kubernetes.io/instance"] = kp.sessionID
			labels["mcp-gateway.docker.com/session"] = kp.sessionID
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: kp.namespace,
				Labels:    labels,
			},
			Type:       corev1.SecretTypeOpaque,
			StringData: secretData,
		}

		// Create or update the secret
		existingSecret, err := clientset.CoreV1().Secrets(kp.namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// Secret doesn't exist, create it
				_, err := clientset.CoreV1().Secrets(kp.namespace).Create(ctx, secret, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create secret %s: %w", secretName, err)
				}
				kp.debugLog("Successfully created shared Kubernetes Secret:", secretName)
			} else {
				return fmt.Errorf("failed to check secret %s: %w", secretName, err)
			}
		} else {
			// Secret exists, update it
			existingSecret.StringData = secretData
			_, err := clientset.CoreV1().Secrets(kp.namespace).Update(ctx, existingSecret, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update secret %s: %w", secretName, err)
			}
			kp.debugLog("Successfully updated shared Kubernetes Secret:", secretName)
		}
	}

	return nil
}

// createSharedConfigMaps creates the shared Kubernetes ConfigMap once with all server configs
func (kp *KubernetesProvisionerImpl) createSharedConfigMaps(ctx context.Context) error {
	if kp.configManager == nil {
		return nil // No ConfigMaps to create in cluster mode
	}

	configSpecs := kp.configManager.GetConfigSpecs("") // Empty server name gets all configs
	if len(configSpecs) == 0 {
		kp.debugLog("No ConfigMaps to create during initialization")
		return nil
	}

	// Get Kubernetes client from container runtime
	k8sRuntime, ok := kp.containerRuntime.(*runtime.KubernetesContainerRuntime)
	if !ok {
		return fmt.Errorf("container runtime is not KubernetesContainerRuntime")
	}

	clientset, err := k8sRuntime.GetClientset()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes clientset: %w", err)
	}

	for configMapName, configData := range configSpecs {
		kp.debugLog("Creating shared Kubernetes ConfigMap:", configMapName)

		// Add session labels for cleanup
		labels := map[string]string{
			"app.kubernetes.io/managed-by": "mcp-gateway",
			"app.kubernetes.io/component":  "mcp-server-config",
			"app.kubernetes.io/name":       "shared-config",
		}
		if kp.sessionID != "" {
			labels["app.kubernetes.io/instance"] = kp.sessionID
			labels["mcp-gateway.docker.com/session"] = kp.sessionID
		}

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: kp.namespace,
				Labels:    labels,
			},
			Data: configData,
		}

		// Create or update the ConfigMap
		existingConfigMap, err := clientset.CoreV1().ConfigMaps(kp.namespace).Get(ctx, configMapName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// ConfigMap doesn't exist, create it
				_, err := clientset.CoreV1().ConfigMaps(kp.namespace).Create(ctx, configMap, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create ConfigMap %s: %w", configMapName, err)
				}
				kp.debugLog("Successfully created shared Kubernetes ConfigMap:", configMapName)
			} else {
				return fmt.Errorf("failed to check ConfigMap %s: %w", configMapName, err)
			}
		} else {
			// ConfigMap exists, update it
			existingConfigMap.Data = configData
			_, err := clientset.CoreV1().ConfigMaps(kp.namespace).Update(ctx, existingConfigMap, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update ConfigMap %s: %w", configMapName, err)
			}
			kp.debugLog("Successfully updated shared Kubernetes ConfigMap:", configMapName)
		}
	}

	return nil
}

// ApplyToolProviders applies secret and config provider settings to a POCI tool container spec
// This uses the same secret/config provider logic (docker-engine vs cluster) as MCP servers
func (kp *KubernetesProvisionerImpl) ApplyToolProviders(spec *runtime.ContainerSpec, toolName string) {
	kp.debugLog("ApplyToolProviders called for tool:", toolName)

	// Apply secret provider logic (same as buildContainerSpec for MCP servers)
	if kp.secretManager != nil {
		// Docker-engine mode: Use SecretManager to get secretKeyRef mappings
		secretKeySelectors := kp.secretManager.GetSecretKeyRefs(toolName)
		secretKeyRefs := make(map[string]runtime.SecretKeyRef)
		for envName, selector := range secretKeySelectors {
			secretKeyRefs[envName] = runtime.SecretKeyRef{
				Name: selector.Name,
				Key:  selector.Key,
			}
		}
		spec.SecretKeyRefs = secretKeyRefs
		kp.debugLog("Applied docker-engine secret provider for tool:", toolName, "secrets:", len(secretKeyRefs))
	} else if kp.secretProvider == ClusterSecretProvider {
		// Cluster mode: Generate secretKeyRefs directly for pre-existing secrets
		spec.SecretKeyRefs = kp.generateClusterSecretKeyRefs(toolName)
		kp.debugLog("Applied cluster secret provider for tool:", toolName, "secrets:", len(spec.SecretKeyRefs))
	}

	// Apply config provider logic (same as buildContainerSpec for MCP servers)
	if kp.configProvider == DockerEngineConfigProvider && kp.configResolver != nil {
		// Docker-engine mode: Use ConfigResolver for just-in-time template resolution
		resolvedEnv := kp.configResolver.ResolveEnvironment(toolName)
		for envName, envValue := range resolvedEnv {
			spec.Env[envName] = envValue
		}
		kp.debugLog("Applied docker-engine config provider for tool:", toolName, "env vars:", len(resolvedEnv))
	} else if kp.configProvider == ClusterConfigProvider {
		// Cluster mode: Environment variables will be injected from ConfigMap
		if kp.configManager != nil {
			spec.ConfigMapRefs = kp.configManager.GetConfigMapRefs(toolName)
		} else {
			// Use default ConfigMap name for cluster mode
			spec.ConfigMapRefs = []string{kp.configName}
		}
		kp.debugLog("Applied cluster config provider for tool:", toolName, "ConfigMaps:", spec.ConfigMapRefs)
	}
}

// debugLog prints debug messages to stderr only when verbose mode is enabled
func (kp *KubernetesProvisionerImpl) debugLog(args ...any) {
	if kp.verbose {
		prefixedArgs := append([]any{"[KubernetesProvisioner]"}, args...)
		fmt.Fprintln(os.Stderr, prefixedArgs...)
	}
}
