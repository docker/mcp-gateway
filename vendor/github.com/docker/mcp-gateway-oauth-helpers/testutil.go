package oauth

import (
	"fmt"
	"strings"
)

// testLogger captures log messages for test verification
type testLogger struct {
	infos  []string
	warns  []string
	debugs []string
}

func (l *testLogger) Infof(format string, args ...any) {
	l.infos = append(l.infos, fmt.Sprintf(format, args...))
}

func (l *testLogger) Warnf(format string, args ...any) {
	l.warns = append(l.warns, fmt.Sprintf(format, args...))
}

func (l *testLogger) Debugf(format string, args ...any) {
	l.debugs = append(l.debugs, fmt.Sprintf(format, args...))
}

func (l *testLogger) containsInfo(substr string) bool {
	for _, msg := range l.infos {
		if strings.Contains(msg, substr) {
			return true
		}
	}
	return false
}
