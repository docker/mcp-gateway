package desktop

import "sync"

type DockerDesktopPaths struct {
	AdminSettingPath     string
	BackendSocket        string
	RawDockerSocket      string
	ToolsSocket          string
	CredentialHelperPath func() string
}

var Paths = sync.OnceValue(func() DockerDesktopPaths {
	desktopPaths, err := getDockerDesktopPaths()
	if err != nil {
		panic(err)
	}

	return desktopPaths
})
