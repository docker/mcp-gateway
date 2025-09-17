package eval

import "fmt"

func volumeTarget(value any) string {
	path := fmt.Sprintf("%v", value)
	if isWindowsPath(path) {
		return toLinuxPath(path)
	}
	return path
}
