package serverless

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	serverlessv1alpha1 "github.com/docker/serverless/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const compositionName = "simple-agent-with-mcp"

// K8sClient implements Client using the Kubernetes API directly
type K8sClient struct {
	k8sClient         client.Client
	deployedResources map[string]DeployedResource
}

// NewK8sClient creates a new K8sClient instance
func NewK8sClient() (*K8sClient, error) {
	k8sClient, err := createKubernetesClient()
	if err != nil {
		return nil, err
	}

	return &K8sClient{
		k8sClient:         k8sClient,
		deployedResources: make(map[string]DeployedResource),
	}, nil
}

// DeployMCPServer deploys an MCP server by creating a Kubernetes Composition resource
func (c *K8sClient) DeployMCPServer(ctx context.Context, serverName, configPath, namespace string) (string, error) {
	if namespace == "" {
		namespace = "default"
	}

	// Create a Composition named with the server's name
	composition := &serverlessv1alpha1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      compositionName,
			Namespace: namespace,
		},
	}

	// Use controllerutil.CreateOrUpdate to handle create/update logic
	result, err := controllerutil.CreateOrUpdate(ctx, c.k8sClient, composition, func() error {
		composition.Spec = buildCompositionSpec()
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to create or update Composition: %w", err)
	}

	fmt.Printf("Composition %s %s successfully\n", composition.Name, result)

	c.deployedResources[serverName] = DeployedResource{
		ConfigPath: configPath,
		Namespace:  namespace,
		ServerName: serverName,
	}

	return compositionName, nil
}

// DeleteMCPServer deletes a deployed MCP server by removing the Composition resource
func (c *K8sClient) DeleteMCPServer(ctx context.Context, serverName string) error {
	resource, exists := c.deployedResources[serverName]
	if !exists {
		return nil
	}

	composition := &serverlessv1alpha1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName,
			Namespace: resource.Namespace,
		},
	}

	if err := c.k8sClient.Delete(ctx, composition); err != nil {
		return fmt.Errorf("failed to delete Composition %s: %w", serverName, err)
	}

	delete(c.deployedResources, serverName)
	return nil
}

// WaitForComposition waits for a composition to reach running state using the Kubernetes API
func (c *K8sClient) WaitForComposition(ctx context.Context, compositionName, namespace string, timeout time.Duration) error {
	if namespace == "" {
		namespace = "default"
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Poll every 2 seconds until composition phase is "Running"
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for composition %s to be running", compositionName)
		case <-ticker.C:
			composition := &serverlessv1alpha1.Composition{}
			err := c.k8sClient.Get(ctx, client.ObjectKey{
				Name:      compositionName,
				Namespace: namespace,
			}, composition)
			if err != nil {
				// Composition might not exist yet, continue polling
				continue
			}

			if composition.Status.Phase == "" {
				// No status yet, continue polling
				continue
			}

			if composition.Status.Phase == "Running" {
				return nil
			}

			// Log current status for user visibility
			if composition.Status.Message != "" {
				fmt.Printf("    Status: %s - %s\n", composition.Status.Phase, composition.Status.Message)
			} else {
				fmt.Printf("    Status: %s\n", composition.Status.Phase)
			}
		}
	}
}

// GetServiceEndpoint retrieves the external endpoint for a service using the Kubernetes API
func (c *K8sClient) GetServiceEndpoint(ctx context.Context, serviceName, namespace string) (string, error) {
	if namespace == "" {
		namespace = "default"
	}

	service := &corev1.Service{}
	err := c.k8sClient.Get(ctx, client.ObjectKey{
		Name:      serviceName,
		Namespace: namespace,
	}, service)
	if err != nil {
		return "", fmt.Errorf("failed to get service %s: %w", serviceName, err)
	}

	// Check for LoadBalancer ingress
	if len(service.Status.LoadBalancer.Ingress) > 0 {
		ingress := service.Status.LoadBalancer.Ingress[0]
		if ingress.IP != "" {
			return ingress.IP, nil
		}
		if ingress.Hostname != "" {
			return ingress.Hostname, nil
		}
	}

	return "", fmt.Errorf("no external endpoint found for service %s", serviceName)
}

// CleanupAll removes all deployed resources
func (c *K8sClient) CleanupAll(ctx context.Context) error {
	return c.DeleteMCPServer(ctx, compositionName)
}

// createKubernetesClient creates a controller-runtime client using the current kubectl context
func createKubernetesClient() (client.Client, error) {
	// Get kubeconfig path (same logic as kubectl)
	kubeconfig := getKubeconfigPath()

	// Build config from current context (same as kubectl uses)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	// Create a runtime scheme and add the types we need
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add core/v1 to scheme: %w", err)
	}
	if err := serverlessv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add serverlessv1alpha1 to scheme: %w", err)
	}

	// Create the controller-runtime client
	c, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return c, nil
}

// getKubeconfigPath returns the kubeconfig path using the same logic as kubectl
func getKubeconfigPath() string {
	// 1. Check KUBECONFIG environment variable first
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}

	// 2. Fall back to default location
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".kube", "config")
	}

	// 3. Last resort (shouldn't happen on most systems)
	return ""
}

// buildCompositionSpec builds the composition specification for the MCP server
func buildCompositionSpec() serverlessv1alpha1.CompositionSpec {
	return serverlessv1alpha1.CompositionSpec{
		Services: map[string]serverlessv1alpha1.ServiceConfig{
			"simple-agent": {
				Image:       "175142243308.dkr.ecr.us-west-1.amazonaws.com/billpijewski-eks-sandbox-6-repo/simple-agent:dev@sha256:15e23900e4c29e76ebbf581f9b5ba3adffa4ed6f7c124a894a327cecac563f70",
				Ports:       []string{"8080:8080"},
				Environment: []string{"LLM_PROVIDER=openai"},
				EgressControl: &serverlessv1alpha1.EgressControl{
					Enabled: true,
					Destinations: []serverlessv1alpha1.EgressDestination{
						{
							Name: "openai-api",
							URL:  "https://api.openai.com",
						},
					},
				},
			},
			"mcp-server": {
				Image: "175142243308.dkr.ecr.us-west-1.amazonaws.com/billpijewski-eks-sandbox-6-repo/simple-agent-mcp:dev@sha256:50643553be545c132fcc2e7eaf6e844fb1f9c5d84fb1422dcfd787f6c91f4e2f",
				Ports: []string{"8000:8000"},
				Environment: []string{
					"MCP_PORT=8000",
					"SIMPLE_AGENT_URL=http://simple-agent.serverless:8080",
				},
				EgressControl: &serverlessv1alpha1.EgressControl{
					Enabled: true,
					Destinations: []serverlessv1alpha1.EgressDestination{
						{
							Name: "simple-agent",
							URL:  "http://simple-agent.serverless:8080",
						},
					},
				},
			},
		},
	}
}
