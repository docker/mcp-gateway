package manager

import (
	"context"
	"os"
	"os/exec"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/mcp-gateway/pkg/desktop"
)

// EnvironmentDetector detects Docker environment capabilities
type EnvironmentDetector struct {
	dockerClient *client.Client
}

// NewEnvironmentDetector creates a new environment detector
func NewEnvironmentDetector(dockerClient *client.Client) *EnvironmentDetector {
	return &EnvironmentDetector{dockerClient: dockerClient}
}

// Detect identifies environment capabilities
func (d *EnvironmentDetector) Detect(ctx context.Context) (*EnvironmentCapabilities, error) {
	caps := &EnvironmentCapabilities{}

	// Detect Docker Desktop
	caps.HasDockerDesktop = d.detectDockerDesktop(ctx)

	// Detect Swarm Mode
	caps.HasSwarmMode = d.detectSwarmMode(ctx)

	// Detect credential helper
	caps.HasCredentialHelper = d.detectCredentialHelper()

	// Determine if secure mount is supported
	caps.SupportsSecureMount = caps.HasDockerDesktop || caps.HasSwarmMode

	// Recommend strategy
	caps.RecommendedStrategy = d.recommendStrategy(caps)

	return caps, nil
}

// detectDockerDesktop checks if Docker Desktop is available
func (d *EnvironmentDetector) detectDockerDesktop(ctx context.Context) bool {
	// Check if JFS socket exists
	paths := desktop.Paths()
	_, err := os.Stat(paths.JFSSocket)
	return err == nil
}

// detectSwarmMode checks if Swarm is active
func (d *EnvironmentDetector) detectSwarmMode(ctx context.Context) bool {
	if d.dockerClient == nil {
		return false
	}

	info, err := d.dockerClient.Info(ctx)
	if err != nil {
		return false
	}

	return info.Swarm.LocalNodeState == swarm.LocalNodeStateActive
}

// detectCredentialHelper checks if docker-credential-pass is available
func (d *EnvironmentDetector) detectCredentialHelper() bool {
	// Check if docker-credential-pass is in PATH
	_, err := exec.LookPath("docker-credential-pass")
	return err == nil
}

// recommendStrategy recommends the best strategy based on capabilities
func (d *EnvironmentDetector) recommendStrategy(caps *EnvironmentCapabilities) string {
	switch {
	case caps.HasDockerDesktop:
		return "desktop-label"
	case caps.HasSwarmMode:
		return "swarm"
	case caps.HasCredentialHelper:
		return "credstore"
	default:
		return "file"
	}
}
