package desktop

import (
	"os/exec"
	"path/filepath"

	"github.com/docker/mcp-gateway/pkg/user"
)

func getDockerDesktopPaths() (DockerDesktopPaths, error) {
	home, err := user.HomeDir()
	if err != nil {
		return DockerDesktopPaths{}, err
	}

	data := filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data")
	applicationSupport := "/Library/Application Support/com.docker.docker"

	return DockerDesktopPaths{
		AdminSettingPath:     filepath.Join(applicationSupport, "admin-settings.json"),
		BackendSocket:        filepath.Join(data, "backend.sock"),
		RawDockerSocket:      filepath.Join(data, "docker.raw.sock"),
		ToolsSocket:          filepath.Join(data, "tools.sock"),
		CredentialHelperPath: getCredentialHelperPath,
	}, nil
}

func getCredentialHelperPath() string {
	name := "docker-credential-osxkeychain"
	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	return name
}
