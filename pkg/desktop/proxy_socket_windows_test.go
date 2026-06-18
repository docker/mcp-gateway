//go:build windows
// +build windows

package desktop

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestDockerDesktopProxySocketTransportReturnsNilWhenWindowsPipeMissing(t *testing.T) {
	previousDial := dialHTTPProxyForAvailability
	t.Cleanup(func() {
		dialHTTPProxyForAvailability = previousDial
	})
	dialHTTPProxyForAvailability = func(context.Context) (net.Conn, error) {
		return nil, errors.New("missing pipe")
	}

	if transport := DockerDesktopProxySocketTransport(context.Background()); transport != nil {
		t.Fatalf("expected nil transport, got %T", transport)
	}
}

func TestDesktopProxySocketAvailableWhenWindowsPipeDials(t *testing.T) {
	conn := &availabilityProbeConn{}

	if !desktopProxySocketAvailableWithDial(context.Background(), func(context.Context) (net.Conn, error) {
		return conn, nil
	}) {
		t.Fatal("expected Docker Desktop proxy pipe to be available")
	}
	if !conn.closed {
		t.Fatal("expected availability probe connection to be closed")
	}
}

type availabilityProbeConn struct {
	closed bool
}

func (c *availabilityProbeConn) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (c *availabilityProbeConn) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (c *availabilityProbeConn) Close() error {
	c.closed = true
	return nil
}

func (c *availabilityProbeConn) LocalAddr() net.Addr {
	return availabilityProbeAddr("local")
}

func (c *availabilityProbeConn) RemoteAddr() net.Addr {
	return availabilityProbeAddr("remote")
}

func (c *availabilityProbeConn) SetDeadline(time.Time) error {
	return nil
}

func (c *availabilityProbeConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *availabilityProbeConn) SetWriteDeadline(time.Time) error {
	return nil
}

type availabilityProbeAddr string

func (a availabilityProbeAddr) Network() string {
	return string(a)
}

func (a availabilityProbeAddr) String() string {
	return string(a)
}
