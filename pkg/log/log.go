package log

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var logWriter io.Writer = os.Stderr

// SetLogWriter sets the log output destination
func SetLogWriter(w io.Writer) {
	if w != nil {
		logWriter = w
	}
}

// Log prints a message to the log output
func Log(a ...any) {
	_, _ = fmt.Fprintln(logWriter, a...)
}

// Logf prints a formatted message to the log output
func Logf(format string, a ...any) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	_, _ = fmt.Fprintf(logWriter, format, a...)
}
