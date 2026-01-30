package registryapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	registryapi "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(baseURL string) *client {
	return &client{
		client:  http.DefaultClient,
		baseURL: baseURL,
	}
}

func TestFetchAllServers(t *testing.T) {
	t.Run("single page response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v0/servers", r.URL.Path)
			assert.Equal(t, "latest", r.URL.Query().Get("version"))

			resp := registryapi.ServerListResponse{
				Servers: []registryapi.ServerResponse{
					{Server: registryapi.ServerJSON{Name: "io.example/server1", Version: "1.0.0"}},
					{Server: registryapi.ServerJSON{Name: "io.example/server2", Version: "2.0.0"}},
				},
				Metadata: registryapi.Metadata{Count: 2},
			}
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(resp))
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		servers, err := c.fetchAllServers(context.Background(), "")
		require.NoError(t, err)
		require.Len(t, servers, 2)
		require.Equal(t, "io.example/server1", servers[0].Server.Name)
		require.Equal(t, "io.example/server2", servers[1].Server.Name)
	})

	t.Run("multi-page pagination", func(t *testing.T) {
		var requestCount atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := requestCount.Add(1)
			w.Header().Set("Content-Type", "application/json")

			switch count {
			case 1:
				assert.Empty(t, r.URL.Query().Get("cursor"))
				resp := registryapi.ServerListResponse{
					Servers: []registryapi.ServerResponse{
						{Server: registryapi.ServerJSON{Name: "io.example/page1-server"}},
					},
					Metadata: registryapi.Metadata{NextCursor: "cursor-page2", Count: 1},
				}
				assert.NoError(t, json.NewEncoder(w).Encode(resp))
			case 2:
				assert.Equal(t, "cursor-page2", r.URL.Query().Get("cursor"))
				resp := registryapi.ServerListResponse{
					Servers: []registryapi.ServerResponse{
						{Server: registryapi.ServerJSON{Name: "io.example/page2-server"}},
					},
					Metadata: registryapi.Metadata{NextCursor: "cursor-page3", Count: 1},
				}
				assert.NoError(t, json.NewEncoder(w).Encode(resp))
			case 3:
				assert.Equal(t, "cursor-page3", r.URL.Query().Get("cursor"))
				resp := registryapi.ServerListResponse{
					Servers: []registryapi.ServerResponse{
						{Server: registryapi.ServerJSON{Name: "io.example/page3-server"}},
					},
					Metadata: registryapi.Metadata{Count: 1}, // No NextCursor = last page
				}
				assert.NoError(t, json.NewEncoder(w).Encode(resp))
			}
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		servers, err := c.fetchAllServers(context.Background(), "")
		require.NoError(t, err)
		require.Len(t, servers, 3)
		require.Equal(t, "io.example/page1-server", servers[0].Server.Name)
		require.Equal(t, "io.example/page2-server", servers[1].Server.Name)
		require.Equal(t, "io.example/page3-server", servers[2].Server.Name)
		require.Equal(t, int32(3), requestCount.Load())
	})

	t.Run("with search query", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "weather", r.URL.Query().Get("q"))
			assert.Equal(t, "latest", r.URL.Query().Get("version"))

			resp := registryapi.ServerListResponse{
				Servers: []registryapi.ServerResponse{
					{Server: registryapi.ServerJSON{Name: "io.example/weather-server"}},
				},
				Metadata: registryapi.Metadata{Count: 1},
			}
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(resp))
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		servers, err := c.fetchAllServers(context.Background(), "weather")
		require.NoError(t, err)
		require.Len(t, servers, 1)
		require.Equal(t, "io.example/weather-server", servers[0].Server.Name)
	})

	t.Run("empty result", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := registryapi.ServerListResponse{
				Servers:  []registryapi.ServerResponse{},
				Metadata: registryapi.Metadata{Count: 0},
			}
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(resp))
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		servers, err := c.fetchAllServers(context.Background(), "")
		require.NoError(t, err)
		require.Empty(t, servers)
	})

	t.Run("HTTP error status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		servers, err := c.fetchAllServers(context.Background(), "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected status code: 500")
		require.Nil(t, servers)
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte("not-json"))
			assert.NoError(t, err)
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)
		servers, err := c.fetchAllServers(context.Background(), "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal response")
		require.Nil(t, servers)
	})

	t.Run("server unreachable", func(t *testing.T) {
		c := newTestClient("http://localhost:1") // nothing listening
		servers, err := c.fetchAllServers(context.Background(), "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to execute request")
		require.Nil(t, servers)
	})
}

func TestListServers(t *testing.T) {
	t.Run("caches full listing", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("HOME", tempDir)

		var callCount atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			callCount.Add(1)
			resp := registryapi.ServerListResponse{
				Servers: []registryapi.ServerResponse{
					{Server: registryapi.ServerJSON{Name: "io.example/cached-server"}},
				},
				Metadata: registryapi.Metadata{Count: 1},
			}
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(resp))
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)

		// First call should hit the server
		servers, err := c.ListServers(context.Background(), "")
		require.NoError(t, err)
		require.Len(t, servers, 1)
		require.Equal(t, int32(1), callCount.Load())

		// Second call should use cache
		servers, err = c.ListServers(context.Background(), "")
		require.NoError(t, err)
		require.Len(t, servers, 1)
		require.Equal(t, int32(1), callCount.Load()) // still 1 - used cache
	})

	t.Run("does not cache when query is specified", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("HOME", tempDir)

		var callCount atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			callCount.Add(1)
			resp := registryapi.ServerListResponse{
				Servers: []registryapi.ServerResponse{
					{Server: registryapi.ServerJSON{Name: "io.example/queried"}},
				},
				Metadata: registryapi.Metadata{Count: 1},
			}
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(resp))
		}))
		defer srv.Close()

		c := newTestClient(srv.URL)

		// First call with query
		servers, err := c.ListServers(context.Background(), "search")
		require.NoError(t, err)
		require.Len(t, servers, 1)
		require.Equal(t, int32(1), callCount.Load())

		// Second call with query should hit server again
		servers, err = c.ListServers(context.Background(), "search")
		require.NoError(t, err)
		require.Len(t, servers, 1)
		require.Equal(t, int32(2), callCount.Load())
	})
}
