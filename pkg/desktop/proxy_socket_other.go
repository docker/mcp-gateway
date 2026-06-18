//go:build !windows
// +build !windows

package desktop

import (
	"context"
	"os"
)

func desktopProxySocketAvailable(_ context.Context) bool {
	return desktopProxySocketAvailableAt(Paths().HTTPProxySocket)
}

func desktopProxySocketAvailableAt(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().Type() == os.ModeSocket
}
