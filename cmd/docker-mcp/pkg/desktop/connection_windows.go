package desktop

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

func dialBackend(ctx context.Context) (net.Conn, error) {
	return dial(ctx, Paths().BackendSocket)
}

func dialAuth(ctx context.Context) (net.Conn, error) {
	return dial(ctx, Paths().ToolsSocket)
}

func dialSecrets(ctx context.Context) (net.Conn, error) {
	return dial(ctx, Paths().JFSSocket)
}

func dial(ctx context.Context, path string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, path)
}
