package secret

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/manager"
	"github.com/docker/mcp-gateway/pkg/docker"
)

// Diagnose detects and displays information about the secrets environment
func Diagnose(ctx context.Context, dockerClient docker.Client) error {
	// Create a Docker API client for the manager
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer apiClient.Close()

	// Create a SecretManager to detect the environment
	mgr, err := manager.NewSecretManager(ctx, manager.ReferenceModeOnly, apiClient)
	if err != nil {
		return fmt.Errorf("failed to create secret manager: %w", err)
	}

	// Get environment capabilities
	env, err := mgr.DetectEnvironment(ctx)
	if err != nil {
		return fmt.Errorf("failed to detect environment: %w", err)
	}

	// Display results
	fmt.Println("Secret Environment Diagnostics:")
	fmt.Println("================================")
	fmt.Printf("Docker Desktop:       %s\n", formatBool(env.HasDockerDesktop))
	fmt.Printf("Swarm Mode:           %s\n", formatBool(env.HasSwarmMode))
	fmt.Printf("Credential Helper:    %s\n", formatBool(env.HasCredentialHelper))
	fmt.Printf("Secure Mount Support: %s\n", formatBool(env.SupportsSecureMount))
	fmt.Printf("Recommended Strategy: %s\n", env.RecommendedStrategy)
	fmt.Println()

	// Display recommendations based on environment
	fmt.Println("Recommendations:")
	fmt.Println("----------------")

	if env.SupportsSecureMount {
		fmt.Println("✅ Your environment supports secure secret mounting!")
		fmt.Println("   Secrets will never be exposed to the gateway process.")
	} else {
		fmt.Println("⚠️  No secure mount support detected.")
		fmt.Println("   Consider one of the following:")
		if !env.HasSwarmMode {
			fmt.Println("   • Initialize Docker Swarm: docker swarm init")
		}
		if !env.HasDockerDesktop {
			fmt.Println("   • Install Docker Desktop for enhanced security")
		}
	}

	fmt.Println()

	// Display specific instructions
	switch env.RecommendedStrategy {
	case "swarm":
		fmt.Println("Using: Docker Swarm Secrets")
		fmt.Println("Setup:")
		fmt.Println("  1. Your swarm is already initialized ✅")
		fmt.Println("  2. Create secrets: docker mcp secret set API_KEY=value")
		fmt.Println("  3. Start gateway: docker mcp gateway run")

	case "desktop-label":
		fmt.Println("Using: Docker Desktop")
		fmt.Println("Setup:")
		fmt.Println("  1. Docker Desktop is running ✅")
		fmt.Println("  2. Create secrets: docker mcp secret set API_KEY=value")
		fmt.Println("  3. Start gateway: docker mcp gateway run")

	case "credstore":
		fmt.Println("Using: Docker Credential Store")
		fmt.Println("Setup:")
		fmt.Println("  1. Credential helper detected ✅")
		fmt.Println("  2. Create secrets: docker mcp secret set API_KEY=value")
		fmt.Println("  Note: Secrets will be stored in your system credential store")

	case "file":
		fmt.Println("Using: Local File Storage (Development Only)")
		fmt.Println("Setup:")
		fmt.Println("  1. Create secrets: docker mcp secret set API_KEY=value")
		fmt.Println("  2. Secrets stored in: ~/.docker/mcp/secrets/")
		fmt.Println("  ⚠️  For production, consider enabling Swarm mode or Docker Desktop")
	}

	fmt.Println()
	fmt.Println("For more information, see: docs/LINUX_SECRETS_SUPPORT.md")

	return nil
}

func formatBool(b bool) string {
	if b {
		return "✅ Yes"
	}
	return "❌ No"
}
