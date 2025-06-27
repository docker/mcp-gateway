package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/user"
)

func getDockerDesktopPaths() (DockerDesktopPaths, error) {
	_, err := os.Stat("/run/host-services/backend.sock")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return DockerDesktopPaths{}, err
		}

		home, err := user.HomeDir()
		if err != nil {
			return DockerDesktopPaths{}, err
		}

		// On Linux
		return DockerDesktopPaths{
			AdminSettingPath:     "/usr/share/docker-desktop/admin-settings.json",
			BackendSocket:        filepath.Join(home, ".docker/desktop/backend.sock"),
			JFSSocket:            filepath.Join(home, ".docker/desktop/jfs.sock"),
			ToolsSocket:          filepath.Join(home, ".docker/desktop/tools.sock"),
			CredentialHelperPath: getCredentialHelperPath,
		}, nil
	}

	// Inside LinuxKit
	return DockerDesktopPaths{
		AdminSettingPath:     "/usr/share/docker-desktop/admin-settings.json",
		BackendSocket:        "/run/host-services/backend.sock",
		JFSSocket:            "/run/host-services/jfs.sock",
		ToolsSocket:          "/run/host-services/tools.sock",
		CredentialHelperPath: getCredentialHelperPath,
	}, nil
}

func getCredentialHelperPath() string {
	name := "docker-credential-pass"
	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	return name
}
