package codemode

import (
	"fmt"
	"os"
)

func console() map[string]any {
	return map[string]any{
		"debug": consoleDebug,
		"error": consoleError,
		"info":  consoleInfo,
		"log":   consoleLog,
		"trace": consoleTrace,
		"warn":  consoleWarn,
	}
}

func consoleDebug(args ...any) {
	fmt.Fprintln(os.Stdout, args...)
}

func consoleError(args ...any) {
	fmt.Fprintln(os.Stdout, args...)
}

func consoleInfo(args ...any) {
	fmt.Fprintln(os.Stdout, args...)
}

func consoleLog(args ...any) {
	fmt.Fprintln(os.Stdout, args...)
}

func consoleTrace(args ...any) {
	fmt.Fprintln(os.Stdout, args...)
}

func consoleWarn(args ...any) {
	fmt.Fprintln(os.Stdout, args...)
}
