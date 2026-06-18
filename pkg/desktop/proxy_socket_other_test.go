//go:build !windows
// +build !windows

package desktop

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopProxySocketUnavailableWithoutSocket(t *testing.T) {
	if desktopProxySocketAvailableAt(filepath.Join(t.TempDir(), "httpproxy.sock")) {
		t.Fatal("expected Docker Desktop proxy socket to be unavailable")
	}
}

func TestDesktopProxySocketUnavailableForRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "httpproxy.sock")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if desktopProxySocketAvailableAt(path) {
		t.Fatal("expected regular file to be rejected as Docker Desktop proxy socket")
	}
}

func TestDesktopProxySocketAvailableForUnixSocket(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "mcp-gateway-desktop-proxy-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	path := filepath.Join(dir, "httpproxy.sock")
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	if !desktopProxySocketAvailableAt(path) {
		t.Fatal("expected Unix socket to be accepted as Docker Desktop proxy socket")
	}
}
