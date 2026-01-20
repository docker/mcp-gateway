package commands

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMcpregistryImportCommand(t *testing.T) {
	// Test server that serves the Garmin MCP example JSON
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Serve the example JSON from our tests
		jsonResponse := `{
  "name": "io.github.slimslenderslacks/garmin_mcp",
  "description": "exposes your fitness and health data to Claude and other MCP-compatible clients.",
  "status": "active",
  "repository": {
    "url": "https://github.com/slimslenderslacks/poci",
    "source": "github"
  },
  "version_detail": {
    "version": "0.1.1"
  },
  "packages": [
    {
      "registry_type": "oci",
      "identifier": "jimclark106/gramin_mcp",
      "version": "latest",
      "environment_variables": [
        {
          "description": "Garmin Connect email address",
          "is_required": true,
          "is_secret": true,
          "name": "GARMIN_EMAIL"
        },
        {
          "description": "Garmin Connect password",
          "is_required": true,
          "is_secret": true,
          "name": "GARMIN_PASSWORD"
        }
      ]
    }
  ],
  "_meta": {
    "io.modelcontextprotocol.registry": {
      "id": "a2cda0d4-8160-4734-880f-1c8de2b484a1",
      "published_at": "2025-09-07T04:40:26.882157132Z",
      "updated_at": "2025-09-07T04:40:26.882157132Z",
      "is_latest": true,
      "release_date": "2025-09-07T04:40:26Z"
    }
  }
}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(jsonResponse))
		if err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer testServer.Close()

	// Test the import function
	err := runMcpregistryImport(t.Context(), testServer.URL, nil)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestMcpregistryImportCommand_InvalidURL(t *testing.T) {
	ctx := t.Context()

	// Test invalid URL
	err := runMcpregistryImport(ctx, "not-a-url", nil)
	if err == nil {
		t.Error("Expected error for invalid URL, got none")
	}

	// Test unsupported scheme
	err = runMcpregistryImport(ctx, "ftp://example.com", nil)
	if err == nil {
		t.Error("Expected error for unsupported scheme, got none")
	}
}

func TestMcpregistryImportCommand_HTTPError(t *testing.T) {
	// Test server that returns 404
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	err := runMcpregistryImport(t.Context(), testServer.URL, nil)
	if err == nil {
		t.Error("Expected error for 404 response, got none")
	}
}

func TestMcpregistryImportCommand_InvalidJSON(t *testing.T) {
	// Test server that returns invalid JSON
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("invalid json"))
		if err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer testServer.Close()

	err := runMcpregistryImport(t.Context(), testServer.URL, nil)
	if err == nil {
		t.Error("Expected error for invalid JSON, got none")
	}
}
