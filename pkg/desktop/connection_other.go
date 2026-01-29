//go:build !windows
// +build !windows

package desktop

import (
	"context"
	"net"
)

func dialBackend(ctx context.Context) (net.Conn, error) {
	return dial(ctx, Paths().BackendSocket)
}

func dialAuth(ctx context.Context) (net.Conn, error) {
	return dial(ctx, Paths().ToolsSocket)
}

func dialHTTPProxy(ctx context.Context) (net.Conn, error) {
	return dial(ctx, Paths().HTTPProxySocket)
}

func dial(ctx context.Context, path string) (net.Conn, error) {
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, "unix", path)
}
