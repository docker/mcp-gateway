package manager

import (
	"fmt"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
)

// DesktopLabelStrategy - Uses Docker Desktop x-secret labels
type DesktopLabelStrategy struct {
	SecretName string
	MountPath  string
}

// Apply adds the x-secret label to the container
func (d *DesktopLabelStrategy) Apply(containerConfig *container.Config, hostConfig *container.HostConfig) error {
	if containerConfig.Labels == nil {
		containerConfig.Labels = make(map[string]string)
	}
	containerConfig.Labels[fmt.Sprintf("x-secret:%s", d.SecretName)] = d.MountPath
	return nil
}

// Type returns the strategy type
func (d *DesktopLabelStrategy) Type() string {
	return "desktop-label"
}

// SwarmSecretStrategy - Uses Docker Swarm Secrets
// Note: This strategy stores information about the secret but the actual mount
// happens through Docker Services, not standalone containers.
// For gateway usage, we should create services instead of containers when
// using Swarm secrets.
type SwarmSecretStrategy struct {
	SecretID   string
	SecretName string
	TargetPath string
	Mode       os.FileMode
}

// Apply configures labels to indicate that this container needs secrets
// The actual Swarm secret mount happens via Docker Service API
func (s *SwarmSecretStrategy) Apply(containerConfig *container.Config, hostConfig *container.HostConfig) error {
	if containerConfig.Labels == nil {
		containerConfig.Labels = make(map[string]string)
	}

	// Add metadata about the required secret
	// The gateway will need to use this to create a Service instead of a standalone container
	containerConfig.Labels[fmt.Sprintf("com.docker.mcp.secret.%s.id", s.SecretName)] = s.SecretID
	containerConfig.Labels[fmt.Sprintf("com.docker.mcp.secret.%s.path", s.SecretName)] = s.TargetPath

	return nil
}

// Type returns the strategy type
func (s *SwarmSecretStrategy) Type() string {
	return "swarm"
}

// GetSecretInfo returns information about the secret for use in service spec
func (s *SwarmSecretStrategy) GetSecretInfo() (secretID, secretName, targetPath string, mode os.FileMode) {
	return s.SecretID, s.SecretName, s.TargetPath, s.Mode
}

// TmpfsMountStrategy - Mounts secret via tmpfs (insecure fallback)
type TmpfsMountStrategy struct {
	SecretName  string
	SecretValue string // Only for fallback
	MountPath   string
}

// Apply creates a tmpfs mount for the secret
func (t *TmpfsMountStrategy) Apply(containerConfig *container.Config, hostConfig *container.HostConfig) error {
	// Create tmpfs mount for /run/secrets
	if hostConfig.Mounts == nil {
		hostConfig.Mounts = []mount.Mount{}
	}

	// Check if mount already exists for /run/secrets
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

	// Add label to indicate that the value needs to be written
	if containerConfig.Labels == nil {
		containerConfig.Labels = make(map[string]string)
	}
	containerConfig.Labels[fmt.Sprintf("com.docker.mcp.secret.tmpfs.%s", t.SecretName)] = "pending"

	// Note: The value will be written via docker exec after container starts
	// This is implemented in the gateway when needed
	return nil
}

// Type returns the strategy type
func (t *TmpfsMountStrategy) Type() string {
	return "tmpfs"
}

// GetSecretValue returns the secret value for later writing
func (t *TmpfsMountStrategy) GetSecretValue() string {
	return t.SecretValue
}

