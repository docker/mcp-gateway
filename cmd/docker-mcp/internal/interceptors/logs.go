package interceptors

import (
	"fmt"
	"os"
	"strings"
)

func logf(format string, a ...any) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	_, _ = fmt.Fprintf(os.Stderr, format, a...)
}
