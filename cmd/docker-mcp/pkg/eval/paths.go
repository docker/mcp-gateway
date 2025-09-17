package eval

import (
	"regexp"
	"strings"
)

func toLinuxPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	driveLetterPattern := regexp.MustCompile(`^([a-zA-Z]):/`)
	return driveLetterPattern.ReplaceAllString(path, "/$1/")
}

func isWindowsPath(path string) bool {
	driveLetterPattern := regexp.MustCompile(`^[a-zA-Z]:\\`)
	uncPathPattern := regexp.MustCompile(`^\\\$begin:math:display$^\\$end:math:display$+\$begin:math:display$^\\$end:math:display$+`)
	return driveLetterPattern.MatchString(path) || uncPathPattern.MatchString(path)
}
