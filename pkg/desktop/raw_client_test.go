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
func newTestClient(t *testing.T, handler http.Handler) (*RawClient, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := &RawClient{
		client: func() *http.Client {
			return &http.Client{
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
						return net.Dial("tcp", server.Listener.Addr().String())
					},
				},
			}
		},
		timeout: 10 * 1e9, // 10s
	}
	return client, server
}

func TestGet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"key": "value"})
	})

	client, _ := newTestClient(t, handler)

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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "something broke"})
	})

	client, _ := newTestClient(t, handler)

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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 Not Found"))
	})

	client, _ := newTestClient(t, handler)

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
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	client, _ := newTestClient(t, handler)

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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "bad input"})
	})

	client, _ := newTestClient(t, handler)

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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client, _ := newTestClient(t, handler)

	err := client.Post(context.Background(), "/test", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPost_NilResult_ErrorStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "server error"})
	})

	client, _ := newTestClient(t, handler)

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

	client, _ := newTestClient(t, handler)

	err := client.Delete(context.Background(), "/apps/test-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDelete_204NoContent(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	client, _ := newTestClient(t, handler)

	err := client.Delete(context.Background(), "/apps/test-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDelete_ErrorStatusJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "provider `cloudflare-autorag`: could not revoke token",
		})
	})

	client, _ := newTestClient(t, handler)

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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 Not Found"))
	})

	client, _ := newTestClient(t, handler)

	err := client.Delete(context.Background(), "/apps/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "HTTP 404: 404 Not Found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
