package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/user"
)

func getDockerDesktopPaths() (DockerDesktopPaths, error) {
	home, err := user.HomeDir()
	if err != nil {
		return DockerDesktopPaths{}, err
	}

	// Check for devhome override via environment variable
	// e.g., DOCKER_DEVHOME=yellow uses ~/devhome/yellow/...
	devhome := os.Getenv("DOCKER_DEVHOME")
	var data string
	if devhome != "" {
		// Use devhome path
		data = filepath.Join(home, "devhome", devhome, "Library", "Containers", "com.docker.docker", "Data")
	} else {
		// Use default path
		data = filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data")
	}
	
	// Also support direct socket path override
	if toolsSocket := os.Getenv("DOCKER_TOOLS_SOCKET"); toolsSocket != "" {
		// Expand ~ to home directory
		if strings.HasPrefix(toolsSocket, "~/") {
			toolsSocket = filepath.Join(home, toolsSocket[2:])
		}
		data = filepath.Dir(toolsSocket)
	}

	applicationSupport := "/Library/Application Support/com.docker.docker"

	return DockerDesktopPaths{
		AdminSettingPath:     filepath.Join(applicationSupport, "admin-settings.json"),
		BackendSocket:        filepath.Join(data, "backend.sock"),
		RawDockerSocket:      filepath.Join(data, "docker.raw.sock"),
		JFSSocket:            filepath.Join(data, "jfs.sock"),
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
