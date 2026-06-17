package gateway

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/docker/mcp-gateway/pkg/user"
)

// MCP_GATEWAY_DOCKER_BIND_ALLOWED_PATHS adds trusted host-path roots to the
// default temp-only allowlist. Use the OS path-list separator.
const dockerBindAllowedPathsEnv = "MCP_GATEWAY_DOCKER_BIND_ALLOWED_PATHS"

type dockerVolumeBind struct {
	raw        string
	source     string
	hostPath   bool
	readOnly   bool
	sourcePath string
}

func validateDockerVolumeBinds(volumes []string) error {
	allowedRoots := dockerBindAllowedRoots()
	for _, raw := range volumes {
		bind, err := parseDockerVolumeBind(raw)
		if err != nil {
			return err
		}
		if !bind.hostPath {
			continue
		}
		if !bind.readOnly {
			return fmt.Errorf("unsafe docker volume %q: host path bind mounts must be read-only", bind.raw)
		}
		if disallowed, reason := disallowedDockerHostPath(bind.sourcePath); disallowed {
			return fmt.Errorf("unsafe docker volume %q: host path %q is blocked (%s)", bind.raw, bind.source, reason)
		}
		if !isPathUnderAnyRoot(bind.sourcePath, allowedRoots) {
			return fmt.Errorf("unsafe docker volume %q: host path %q is outside allowed roots %s",
				bind.raw, bind.source, strings.Join(allowedRoots, ", "))
		}
	}
	return nil
}

func parseDockerVolumeBind(raw string) (dockerVolumeBind, error) {
	spec := strings.TrimSpace(raw)
	if spec == "" {
		return dockerVolumeBind{}, fmt.Errorf("invalid docker volume %q: empty volume spec", raw)
	}

	parts := splitDockerVolumeSpec(spec)
	if len(parts) > 3 {
		return dockerVolumeBind{}, fmt.Errorf("invalid docker volume %q: expected source:target[:mode]", raw)
	}

	bind := dockerVolumeBind{raw: spec}
	switch len(parts) {
	case 1:
		// A single path is an anonymous Docker volume target, not a host bind.
		return bind, nil
	case 2, 3:
		bind.source = strings.TrimSpace(parts[0])
		if bind.source == "" || strings.TrimSpace(parts[1]) == "" {
			return dockerVolumeBind{}, fmt.Errorf("invalid docker volume %q: source and target are required", raw)
		}
		if len(parts) == 3 {
			bind.readOnly = dockerVolumeModeReadOnly(parts[2])
		}
		bind.hostPath = dockerVolumeSourceIsHostPath(bind.source)
		if bind.hostPath {
			sourcePath, err := cleanDockerHostPath(bind.source)
			if err != nil {
				return dockerVolumeBind{}, fmt.Errorf("invalid docker volume %q: resolve host path %q: %w", raw, bind.source, err)
			}
			bind.sourcePath = sourcePath
		}
	}

	return bind, nil
}

func splitDockerVolumeSpec(spec string) []string {
	var parts []string
	start := 0
	for i := range spec {
		if spec[i] != ':' {
			continue
		}
		if isWindowsDriveColon(spec, start, i) {
			continue
		}
		parts = append(parts, spec[start:i])
		start = i + 1
	}
	return append(parts, spec[start:])
}

func isWindowsDriveColon(spec string, partStart, colon int) bool {
	return colon == partStart+1 &&
		colon+1 < len(spec) &&
		isASCIIAlpha(rune(spec[partStart])) &&
		(spec[colon+1] == '\\' || spec[colon+1] == '/')
}

func isASCIIAlpha(r rune) bool {
	return r <= unicode.MaxASCII && unicode.IsLetter(r)
}

func dockerVolumeModeReadOnly(mode string) bool {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return false
	}

	readOnly := false
	for opt := range strings.SplitSeq(mode, ",") {
		switch strings.ToLower(strings.TrimSpace(opt)) {
		case "rw":
			return false
		case "ro", "readonly":
			readOnly = true
		}
	}
	return readOnly
}

func dockerVolumeSourceIsHostPath(source string) bool {
	source = strings.TrimSpace(source)
	if source == "" {
		return false
	}
	return strings.HasPrefix(source, "/") ||
		strings.HasPrefix(source, "\\\\") ||
		strings.HasPrefix(source, "./") ||
		strings.HasPrefix(source, "../") ||
		source == "." ||
		source == ".." ||
		strings.HasPrefix(source, "~/") ||
		isWindowsAbsPath(source)
}

func isWindowsAbsPath(p string) bool {
	return len(p) >= 3 &&
		isASCIIAlpha(rune(p[0])) &&
		p[1] == ':' &&
		(p[2] == '\\' || p[2] == '/')
}

func cleanDockerHostPath(p string) (string, error) {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if isWindowsAbsPath(p) {
		return strings.ToLower(path.Clean(p)), nil
	}
	if strings.HasPrefix(p, "//") {
		return "//" + path.Clean(strings.TrimPrefix(p, "//")), nil
	}
	if strings.HasPrefix(p, "~/") {
		home, err := user.HomeDir()
		if err != nil {
			return "", fmt.Errorf("expand home directory: %w", err)
		}
		if home == "" {
			return "", fmt.Errorf("expand home directory: empty home directory")
		}
		p = path.Join(filepath.ToSlash(home), strings.TrimPrefix(p, "~/"))
	}

	fsPath := filepath.FromSlash(path.Clean(p))
	if !filepath.IsAbs(fsPath) {
		absPath, err := filepath.Abs(fsPath)
		if err != nil {
			return "", err
		}
		fsPath = absPath
	}
	resolved, err := resolveDockerHostPath(fsPath)
	if err != nil {
		return "", err
	}
	return path.Clean(filepath.ToSlash(resolved)), nil
}

func resolveDockerHostPath(p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	existing := p
	var missing []string
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		missing = append([]string{filepath.Base(existing)}, missing...)
		existing = parent
	}

	resolvedExisting, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{resolvedExisting}, missing...)...), nil
}

func dockerBindAllowedRoots() []string {
	roots := []string{"/tmp", "/private/tmp", "/var/tmp"}
	if tmp := os.TempDir(); tmp != "" {
		roots = append(roots, tmp)
	}
	if env := os.Getenv(dockerBindAllowedPathsEnv); env != "" {
		roots = append(roots, filepath.SplitList(env)...)
	}

	out := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		cleaned, err := cleanDockerHostPath(root)
		if err != nil {
			continue
		}
		if cleaned == "." || cleaned == "/" || slices.Contains(out, cleaned) {
			continue
		}
		out = append(out, cleaned)
	}
	slices.Sort(out)
	return out
}

func disallowedDockerHostPath(p string) (bool, string) {
	lower := strings.ToLower(p)
	for _, blocked := range []string{
		"/dev",
		"/etc",
		"/library",
		"/proc",
		"/private/etc",
		"/private/var/run",
		"/root",
		"/run",
		"/sys",
		"/system",
		"/var/lib/containerd",
		"/var/lib/docker",
		"/var/root",
		"/var/run",
		"c:/windows",
	} {
		if pathHasPrefix(lower, blocked) {
			return true, "sensitive system path"
		}
	}

	for _, segment := range strings.Split(lower, "/") {
		switch segment {
		case ".aws", ".azure", ".config", ".docker", ".gnupg", ".kube", ".ssh":
			return true, "credential path"
		case ".netrc", ".npmrc", "id_dsa", "id_ecdsa", "id_ed25519", "id_rsa":
			return true, "credential file"
		}
	}

	return false, ""
}

func isPathUnderAnyRoot(p string, roots []string) bool {
	for _, root := range roots {
		if pathHasPrefix(p, root) {
			return true
		}
	}
	return false
}

func pathHasPrefix(p, root string) bool {
	p = strings.TrimRight(p, "/")
	root = strings.TrimRight(root, "/")
	return p == root || strings.HasPrefix(p, root+"/")
}
