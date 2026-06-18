//go:build windows
// +build windows

package desktop

import (
	"context"
	"net"
	"time"
)

const desktopProxySocketProbeTimeout = 500 * time.Millisecond

var dialHTTPProxyForAvailability = func(ctx context.Context) (net.Conn, error) {
	return dialHTTPProxy(ctx)
}

func desktopProxySocketAvailable(ctx context.Context) bool {
	return desktopProxySocketAvailableWithDial(ctx, dialHTTPProxyForAvailability)
}

func desktopProxySocketAvailableWithDial(ctx context.Context, dialer func(context.Context) (net.Conn, error)) bool {
	if ctx == nil {
		ctx = context.Background()
	}

	probeCtx, cancel := context.WithTimeout(ctx, desktopProxySocketProbeTimeout)
	defer cancel()

	conn, err := dialer(probeCtx)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
