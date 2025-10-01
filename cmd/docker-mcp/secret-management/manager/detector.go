package manager

import (
	"context"
	"os"
	"os/exec"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/mcp-gateway/pkg/desktop"
)

// EnvironmentDetector detecta capacidades do ambiente Docker
type EnvironmentDetector struct {
	dockerClient *client.Client
}

// NewEnvironmentDetector cria um novo detector de ambiente
func NewEnvironmentDetector(dockerClient *client.Client) *EnvironmentDetector {
	return &EnvironmentDetector{dockerClient: dockerClient}
}

// Detect identifica as capacidades do ambiente
func (d *EnvironmentDetector) Detect(ctx context.Context) (*EnvironmentCapabilities, error) {
	caps := &EnvironmentCapabilities{}

	// Detecta Docker Desktop
	caps.HasDockerDesktop = d.detectDockerDesktop(ctx)

	// Detecta Swarm Mode
	caps.HasSwarmMode = d.detectSwarmMode(ctx)

	// Detecta credential helper
	caps.HasCredentialHelper = d.detectCredentialHelper()

	// Determina se suporta montagem segura
	caps.SupportsSecureMount = caps.HasDockerDesktop || caps.HasSwarmMode

	// Recomenda estratégia
	caps.RecommendedStrategy = d.recommendStrategy(caps)

	return caps, nil
}

// detectDockerDesktop verifica se Docker Desktop está disponível
func (d *EnvironmentDetector) detectDockerDesktop(ctx context.Context) bool {
	// Verifica se JFS socket existe
	paths := desktop.Paths()
	_, err := os.Stat(paths.JFSSocket)
	return err == nil
}

// detectSwarmMode verifica se Swarm está ativo
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

// detectCredentialHelper verifica se docker-credential-pass está disponível
func (d *EnvironmentDetector) detectCredentialHelper() bool {
	// Verifica se docker-credential-pass está no PATH
	_, err := exec.LookPath("docker-credential-pass")
	return err == nil
}

// recommendStrategy recomenda a melhor estratégia baseado nas capacidades
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
