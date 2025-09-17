package gateway

import (
	"fmt"
	"os"
	"strings"
)

func log(a ...any) {
	_, _ = fmt.Fprintln(os.Stderr, a...)
}

func logf(format string, a ...any) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	_, _ = fmt.Fprintf(os.Stderr, format, a...)
}
