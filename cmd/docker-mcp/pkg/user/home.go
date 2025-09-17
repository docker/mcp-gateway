package user

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/docker/cli/cli/config"
)

func HomeDir() (string, error) {
	//nolint:forbidigo
	home, err := os.UserHomeDir()
	if err != nil {
		// Probably HOME/USERPROFILE environment variable is hidden by the MCP client to improve security.

		// On Darwin/Linux, user.Current() might work
		if home == "" && runtime.GOOS != "windows" {
			if u, err := user.Current(); err == nil {
				return u.HomeDir, nil
			}
		}

		// Or, we can assume the current binary (docker-mcp) is in the standard docker cli plugins location
		// and derive the home directory from there.
		return filepath.Dir(config.Dir()), nil
	}

	return home, nil
}
