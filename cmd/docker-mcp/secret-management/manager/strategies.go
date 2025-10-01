package manager

import (
	"fmt"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
)

// DesktopLabelStrategy - Usa labels x-secret do Docker Desktop
type DesktopLabelStrategy struct {
	SecretName string
	MountPath  string
}

// Apply adiciona a label x-secret ao container
func (d *DesktopLabelStrategy) Apply(containerConfig *container.Config, hostConfig *container.HostConfig) error {
	if containerConfig.Labels == nil {
		containerConfig.Labels = make(map[string]string)
	}
	containerConfig.Labels[fmt.Sprintf("x-secret:%s", d.SecretName)] = d.MountPath
	return nil
}

// Type retorna o tipo de estratégia
func (d *DesktopLabelStrategy) Type() string {
	return "desktop-label"
}

// SwarmSecretStrategy - Usa Docker Swarm Secrets
// Nota: Esta estratégia armazena informações sobre o secret mas a montagem
// real acontece através de Docker Services, não containers standalone.
// Para uso no gateway, devemos criar services ao invés de containers quando
// usando Swarm secrets.
type SwarmSecretStrategy struct {
	SecretID   string
	SecretName string
	TargetPath string
	Mode       os.FileMode
}

// Apply configura labels para indicar que este container precisa de secrets
// A montagem real do Swarm secret acontece via Docker Service API
func (s *SwarmSecretStrategy) Apply(containerConfig *container.Config, hostConfig *container.HostConfig) error {
	if containerConfig.Labels == nil {
		containerConfig.Labels = make(map[string]string)
	}
	
	// Adiciona metadata sobre o secret necessário
	// O gateway precisará usar isto para criar um Service ao invés de container standalone
	containerConfig.Labels[fmt.Sprintf("com.docker.mcp.secret.%s.id", s.SecretName)] = s.SecretID
	containerConfig.Labels[fmt.Sprintf("com.docker.mcp.secret.%s.path", s.SecretName)] = s.TargetPath
	
	return nil
}

// Type retorna o tipo de estratégia
func (s *SwarmSecretStrategy) Type() string {
	return "swarm"
}

// GetSecretInfo retorna informações sobre o secret para uso em service spec
func (s *SwarmSecretStrategy) GetSecretInfo() (secretID, secretName, targetPath string, mode os.FileMode) {
	return s.SecretID, s.SecretName, s.TargetPath, s.Mode
}

// TmpfsMountStrategy - Monta secret via tmpfs (fallback inseguro)
type TmpfsMountStrategy struct {
	SecretName  string
	SecretValue string // Apenas para fallback
	MountPath   string
}

// Apply cria um tmpfs mount para o secret
func (t *TmpfsMountStrategy) Apply(containerConfig *container.Config, hostConfig *container.HostConfig) error {
	// Cria tmpfs mount para /run/secrets
	if hostConfig.Mounts == nil {
		hostConfig.Mounts = []mount.Mount{}
	}

	// Verifica se já existe mount para /run/secrets
	hasSecretsMount := false
	for _, m := range hostConfig.Mounts {
		if m.Target == "/run/secrets" {
			hasSecretsMount = true
			break
		}
	}

	if !hasSecretsMount {
		hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
			Type:   mount.TypeTmpfs,
			Target: "/run/secrets",
			TmpfsOptions: &mount.TmpfsOptions{
				SizeBytes: 1024 * 1024, // 1MB
				Mode:      0400,
			},
		})
	}

	// Adiciona label para indicar que o valor precisa ser escrito
	if containerConfig.Labels == nil {
		containerConfig.Labels = make(map[string]string)
	}
	containerConfig.Labels[fmt.Sprintf("com.docker.mcp.secret.tmpfs.%s", t.SecretName)] = "pending"

	// Nota: O valor será escrito via docker exec após o container iniciar
	// Isso é implementado no gateway quando necessário
	return nil
}

// Type retorna o tipo de estratégia
func (t *TmpfsMountStrategy) Type() string {
	return "tmpfs"
}

// GetSecretValue retorna o valor do secret para escrita posterior
func (t *TmpfsMountStrategy) GetSecretValue() string {
	return t.SecretValue
}

