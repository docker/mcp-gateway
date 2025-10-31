package terminal

import (
	"os"

	"github.com/moby/term"
)

// GetWidth returns the width of the terminal in characters.
// If the width cannot be determined, it returns a default value of 120.
func GetWidth() int {
	fd, _ := term.GetFdInfo(os.Stdout)
	ws, err := term.GetWinsize(fd)
	if err != nil {
		return 120
	}
	return int(ws.Width)
}

// GetWidthFrom returns the width of the terminal from a given output stream.
// If the width cannot be determined from the provided stream, it falls back to os.Stdout.
// If that also fails, it returns a default value of 120.
func GetWidthFrom(out interface{}) int {
	// Try to get fd from the provided output
	fd, _ := term.GetFdInfo(out)
	if fd == 0 {
		// If that didn't work, try stdout directly
		fd, _ = term.GetFdInfo(os.Stdout)
	}
	ws, err := term.GetWinsize(fd)
	if err != nil {
		return 120
	}
	return int(ws.Width)
}
