package serverless

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// compositionName needs to match the name in the Composition's YAML
const yamlCompositionName = "simple-agent-with-mcp"
// LocalYamlClient implements Client using kubectl with YAML files
type LocalYamlClient struct {
	deployedResources map[string]DeployedResource
}

// NewLocalYamlClient creates a new LocalYamlClient instance
func NewLocalYamlClient() *LocalYamlClient {
	return &LocalYamlClient{
		deployedResources: make(map[string]DeployedResource),
	}
}

// DeployMCPServer deploys an MCP server using kubectl apply with YAML files
func (c *LocalYamlClient) DeployMCPServer(ctx context.Context, serverName, configPath, namespace string) (string, error) {
	if namespace == "" {
		namespace = "default"
	}

	args := []string{"apply", "-f", configPath}
	if namespace != "default" {
		args = append(args, "-n", namespace)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl apply failed for %s: %w\nOutput: %s", configPath, err, string(output))
	}

	c.deployedResources[serverName] = DeployedResource{
		ConfigPath: configPath,
		Namespace:  namespace,
		ServerName: serverName,
	}

	return yamlCompositionName, nil
}

// DeleteMCPServer deletes a deployed MCP server using kubectl delete
func (c *LocalYamlClient) DeleteMCPServer(ctx context.Context, serverName string) error {
	resource, exists := c.deployedResources[serverName]
	if !exists {
		return nil
	}

	args := []string{"delete", "-f", resource.ConfigPath}
	if resource.Namespace != "default" {
		args = append(args, "-n", resource.Namespace)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kubectl delete failed for %s: %w\nOutput: %s", resource.ConfigPath, err, string(output))
	}

	delete(c.deployedResources, serverName)
	return nil
}

// WaitForComposition waits for a composition to reach running state using kubectl polling
func (c *LocalYamlClient) WaitForComposition(ctx context.Context, compositionName, namespace string, timeout time.Duration) error {
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
			// Get phase and message
			args := []string{
				"get", "composition", compositionName, "-n", namespace,
				"-o", "jsonpath={.status.phase} - {.status.message}",
			}

			cmd := exec.CommandContext(ctx, "kubectl", args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				// Composition might not exist yet, continue polling
				continue
			}

			status := strings.TrimSpace(string(output))
			if status == "" {
				// No status yet, continue polling
				continue
			}

			// Parse phase from "Running - All 2 services are running" format
			parts := strings.SplitN(status, " - ", 2)
			phase := parts[0]

			if phase == "Running" {
				return nil
			}

			// Log current status for user visibility
			if len(parts) > 1 {
				fmt.Printf("    Status: %s - %s\n", phase, parts[1])
			} else {
				fmt.Printf("    Status: %s\n", phase)
			}
		}
	}
}

// GetServiceEndpoint retrieves the external endpoint for a service using kubectl
func (c *LocalYamlClient) GetServiceEndpoint(ctx context.Context, serviceName, namespace string) (string, error) {
	if namespace == "" {
		namespace = "default"
	}

	args := []string{
		"get", "service", serviceName, "-n", namespace,
		"-o", "jsonpath={.status.loadBalancer.ingress[0].ip}",
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("getting service endpoint failed: %w\nOutput: %s", err, string(output))
	}

	endpoint := strings.TrimSpace(string(output))
	if endpoint == "" {
		args = []string{
			"get", "service", serviceName, "-n", namespace,
			"-o", "jsonpath={.status.loadBalancer.ingress[0].hostname}",
		}
		cmd = exec.CommandContext(ctx, "kubectl", args...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("getting service hostname failed: %w\nOutput: %s", err, string(output))
		}
		endpoint = strings.TrimSpace(string(output))
	}

	if endpoint == "" {
		return "", fmt.Errorf("no external endpoint found for service %s", serviceName)
	}

	return endpoint, nil
}

// CleanupAll removes all deployed resources
func (c *LocalYamlClient) CleanupAll(ctx context.Context) error {
	var errors []string

	for serverName := range c.deployedResources {
		if err := c.DeleteMCPServer(ctx, serverName); err != nil {
			errors = append(errors, fmt.Sprintf("failed to cleanup %s: %v", serverName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}

	return nil
}
