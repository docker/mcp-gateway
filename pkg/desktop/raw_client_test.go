package desktop

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient creates a RawClient that connects to an httptest.Server
func newTestClient(t *testing.T, handler http.Handler) *RawClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := &RawClient{
		client: func() *http.Client {
			return &http.Client{
				Transport: &http.Transport{
					DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
						var d net.Dialer
						return d.DialContext(context.Background(), "tcp", server.Listener.Addr().String())
					},
				},
			}
		},
		timeout: 10 * 1e9, // 10s
	}
	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("failed to encode JSON response: %v", err)
	}
}

func TestGet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, map[string]string{"key": "value"})
	})

	client := newTestClient(t, handler)

	var result map[string]string
	err := client.Get(context.Background(), "/test", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got key=%s", result["key"])
	}
}

func TestGet_ErrorStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(t, w, map[string]string{"message": "something broke"})
	})

	client := newTestClient(t, handler)

	var result map[string]string
	err := client.Get(context.Background(), "/test", &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "HTTP 500: something broke"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestGet_ErrorStatusPlainBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte("404 Not Found")); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	})

	client := newTestClient(t, handler)

	var result map[string]string
	err := client.Get(context.Background(), "/test", &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "HTTP 404: 404 Not Found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestPost_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, map[string]string{"status": "ok"})
	})

	client := newTestClient(t, handler)

	var result map[string]string
	err := client.Post(context.Background(), "/test", map[string]string{"input": "data"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got status=%s", result["status"])
	}
}

func TestPost_ErrorStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(t, w, map[string]string{"message": "bad input"})
	})

	client := newTestClient(t, handler)

	var result map[string]string
	err := client.Post(context.Background(), "/test", map[string]string{"input": "data"}, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "HTTP 400: bad input"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestPost_NilResult_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := newTestClient(t, handler)

	err := client.Post(context.Background(), "/test", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPost_NilResult_ErrorStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(t, w, map[string]string{"message": "server error"})
	})

	client := newTestClient(t, handler)

	err := client.Post(context.Background(), "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "HTTP 500: server error"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestDelete_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/apps/test-app" {
			t.Errorf("expected path /apps/test-app, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	client := newTestClient(t, handler)

	err := client.Delete(context.Background(), "/apps/test-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDelete_204NoContent(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	client := newTestClient(t, handler)

	err := client.Delete(context.Background(), "/apps/test-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDelete_ErrorStatusJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(t, w, map[string]string{
			"message": "provider `cloudflare-autorag`: could not revoke token",
		})
	})

	client := newTestClient(t, handler)

	err := client.Delete(context.Background(), "/apps/cloudflare-autorag")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "HTTP 500: provider `cloudflare-autorag`: could not revoke token"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestDelete_ErrorStatusPlainBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte("404 Not Found")); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	})

	client := newTestClient(t, handler)

	err := client.Delete(context.Background(), "/apps/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "HTTP 404: 404 Not Found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
