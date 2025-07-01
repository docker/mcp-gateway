package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

func getDockerDesktopPaths() (DockerDesktopPaths, error) {
	appData := os.Getenv("ProgramData")
	if appData == "" {
		return DockerDesktopPaths{}, errors.New("unable to get 'ProgramData'")
	}

	return DockerDesktopPaths{
		AdminSettingPath:     filepath.Join(appData, `DockerDesktop\admin-settings.json`),
		BackendSocket:        `\\.\pipe\dockerBackendApiServer`,
		RawDockerSocket:      `\\.\pipe\docker_engine_linux`,
		JFSSocket:            `\\.\pipe\dockerJfs`,
		ToolsSocket:          `\\.\pipe\dockerTools`,
		CredentialHelperPath: getCredentialHelperPath,
	}, nil
}

func getCredentialHelperPath() string {
	name := "docker-credential-wincred.exe"
	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	return name
}
