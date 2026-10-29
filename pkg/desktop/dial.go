package desktop

import (
	"context"
	"net"
)

// DialBackend opens the Docker Desktop backend socket.
// Windows uses the named pipe; other platforms use the Unix socket.
func DialBackend(ctx context.Context) (net.Conn, error) {
	return dialBackend(ctx)
}
