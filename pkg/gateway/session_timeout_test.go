package gateway

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// startTestStreamingGateway serves the streaming transport on a random
// loopback port. It returns the MCP endpoint and a channel that is closed when
// a session's cleanup hook runs, mirroring the InitializedHandler in run.go.
func startTestStreamingGateway(t *testing.T, sessionTimeout time.Duration) (string, <-chan struct{}) {
	t.Helper()

	released := make(chan struct{})
	g := &Gateway{
		Options: Options{
			AllowUnauthenticated: true,
			SessionTimeout:       sessionTimeout,
		},
	}
	g.clientPool = newClientPool(g.Options, nil, g)
	g.mcpServer = mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, &mcp.ServerOptions{
		InitializedHandler: func(_ context.Context, req *mcp.InitializedRequest) {
			ss := req.Session
			go func() {
				_ = ss.Wait()
				g.clientPool.ReleaseClientsForSession(ss)
				close(released)
			}()
		},
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = g.startStreamingServer(ctx, ln)
	}()
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
		<-done
	})

	return "http://" + ln.Addr().String() + "/mcp", released
}

func postMCP(t *testing.T, endpoint, sessionID, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp
}

// initializeSession performs the MCP handshake and returns the session ID the
// gateway assigned.
func initializeSession(t *testing.T, endpoint string) string {
	t.Helper()

	resp := postMCP(t, endpoint, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"session-timeout-test","version":"0"}}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize: status %d", resp.StatusCode)
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize: no Mcp-Session-Id header")
	}

	resp = postMCP(t, endpoint, sessionID, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notification: status %d", resp.StatusCode)
	}

	return sessionID
}

func pingSession(t *testing.T, endpoint, sessionID string) int {
	t.Helper()
	return postMCP(t, endpoint, sessionID, `{"jsonrpc":"2.0","id":2,"method":"ping"}`).StatusCode
}

func waitClosed(t *testing.T, ch <-chan struct{}, wait time.Duration) bool {
	t.Helper()
	select {
	case <-ch:
		return true
	case <-time.After(wait):
		return false
	}
}

// TestStreamingSessionIdleTimeout checks that a streaming session which stops
// sending requests is closed after SessionTimeout, that the session's cleanup
// hook runs so cached upstream clients get released, and that a zero timeout
// leaves the session alone.
func TestStreamingSessionIdleTimeout(t *testing.T) {
	const timeout = 150 * time.Millisecond
	// Long enough that the timer must have fired, short enough to keep the
	// suite fast.
	idle := 4 * timeout

	t.Run("enabled", func(t *testing.T) {
		endpoint, released := startTestStreamingGateway(t, timeout)
		sessionID := initializeSession(t, endpoint)

		if code := pingSession(t, endpoint, sessionID); code != http.StatusOK {
			t.Fatalf("ping before the idle period: status %d, want %d", code, http.StatusOK)
		}

		if !waitClosed(t, released, 10*time.Second) {
			t.Fatal("cleanup hook never ran: timeout did not close the idle session")
		}

		if code := pingSession(t, endpoint, sessionID); code != http.StatusNotFound {
			t.Fatalf("ping after the idle period: status %d, want %d (session should have been closed)", code, http.StatusNotFound)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		endpoint, released := startTestStreamingGateway(t, 0)
		sessionID := initializeSession(t, endpoint)

		if code := pingSession(t, endpoint, sessionID); code != http.StatusOK {
			t.Fatalf("ping before the idle period: status %d, want %d", code, http.StatusOK)
		}

		time.Sleep(idle)

		if waitClosed(t, released, 0) {
			t.Fatal("cleanup hook ran even though the timeout is disabled")
		}
		if code := pingSession(t, endpoint, sessionID); code != http.StatusOK {
			t.Fatalf("ping after the idle period with no timeout: status %d, want %d", code, http.StatusOK)
		}
	})
}
