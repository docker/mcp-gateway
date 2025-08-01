package sockets

import (
	"context"
	"net"
)

// AcceptWithContext accepts a connection or returns an error if the context is done. Note the caller still owns the listener and is responsible for closing it.
func AcceptWithContext(ctx context.Context, l net.Listener) (net.Conn, error) {
	accepted := make(chan bool, 1)
	go func() {
		select {
		case <-accepted:
			return
		case <-ctx.Done():
			// Unblock the Accept by connecting to it
			if c, err := unblock(l); err == nil {
				_ = c.Close()
			}
		}
	}()
	c, err := l.Accept()
	accepted <- true
	select {
	case <-ctx.Done():
		if c != nil {
			_ = c.Close()
		}
		return nil, ctx.Err()
	default:
	}
	return c, err
}

func unblock(l net.Listener) (net.Conn, error) {
	dialer := &net.Dialer{}
	return dialer.DialContext(context.Background(), "tcp", l.Addr().String())
}
