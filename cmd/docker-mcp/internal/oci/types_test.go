package oci

import (
	"encoding/json"
	"testing"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

func TestServerDetailParsing(t *testing.T) {
	// Test data matching the provided JSON structure
	jsonData := `{
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

	var serverDetail ServerDetail
	err := json.Unmarshal([]byte(jsonData), &serverDetail)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify basic fields
	if serverDetail.Name != "io.github.slimslenderslacks/garmin_mcp" {
		t.Errorf("Expected name 'io.github.slimslenderslacks/garmin_mcp', got '%s'", serverDetail.Name)
	}

	if serverDetail.Description != "exposes your fitness and health data to Claude and other MCP-compatible clients." {
		t.Errorf("Expected description to match, got '%s'", serverDetail.Description)
	}

	if serverDetail.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", serverDetail.Status)
	}

	// Verify version detail
	if serverDetail.VersionDetail == nil {
		t.Error("Expected VersionDetail to be non-nil")
	} else if serverDetail.VersionDetail.Version != "0.1.1" {
		t.Errorf("Expected version '0.1.1', got '%s'", serverDetail.VersionDetail.Version)
	}

	// Verify repository
	if serverDetail.Repository == nil {
		t.Error("Expected Repository to be non-nil")
	} else {
		if serverDetail.Repository.URL != "https://github.com/slimslenderslacks/poci" {
			t.Errorf("Expected repository URL 'https://github.com/slimslenderslacks/poci', got '%s'", serverDetail.Repository.URL)
		}
		if serverDetail.Repository.Source != "github" {
			t.Errorf("Expected repository source 'github', got '%s'", serverDetail.Repository.Source)
		}
	}

	// Verify packages
	if len(serverDetail.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(serverDetail.Packages))
	} else {
		pkg := serverDetail.Packages[0]
		if pkg.RegistryType != "oci" {
			t.Errorf("Expected registry type 'oci', got '%s'", pkg.RegistryType)
		}
		if pkg.Identifier != "jimclark106/gramin_mcp" {
			t.Errorf("Expected identifier 'jimclark106/gramin_mcp', got '%s'", pkg.Identifier)
		}
		if pkg.Version != "latest" {
			t.Errorf("Expected version 'latest', got '%s'", pkg.Version)
		}

		// Verify environment variables
		if len(pkg.Env) != 2 {
			t.Errorf("Expected 2 environment variables, got %d", len(pkg.Env))
		} else {
			// Check first environment variable
			env1 := pkg.Env[0]
			if env1.Name != "GARMIN_EMAIL" {
				t.Errorf("Expected first env var name 'GARMIN_EMAIL', got '%s'", env1.Name)
			}
			if env1.Description != "Garmin Connect email address" {
				t.Errorf("Expected first env var description 'Garmin Connect email address', got '%s'", env1.Description)
			}
			if !env1.Required {
				t.Error("Expected first env var to be required")
			}
			if !env1.Secret {
				t.Error("Expected first env var to be secret")
			}

			// Check second environment variable
			env2 := pkg.Env[1]
			if env2.Name != "GARMIN_PASSWORD" {
				t.Errorf("Expected second env var name 'GARMIN_PASSWORD', got '%s'", env2.Name)
			}
			if env2.Description != "Garmin Connect password" {
				t.Errorf("Expected second env var description 'Garmin Connect password', got '%s'", env2.Description)
			}
			if !env2.Required {
				t.Error("Expected second env var to be required")
			}
			if !env2.Secret {
				t.Error("Expected second env var to be secret")
			}
		}
	}

	// Verify meta field exists
	if serverDetail.Meta == nil {
		t.Error("Expected Meta to be non-nil")
	}
}

func TestServerDetailToCatalogServer(t *testing.T) {
	// Use the same JSON data as the parsing test
	jsonData := `{
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

	var serverDetail ServerDetail
	err := json.Unmarshal([]byte(jsonData), &serverDetail)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Convert to catalog server
	catalogServer := serverDetail.ToCatalogServer()

	// Verify basic conversion
	if catalogServer.Description != "exposes your fitness and health data to Claude and other MCP-compatible clients." {
		t.Errorf("Expected description to match, got '%s'", catalogServer.Description)
	}

	if catalogServer.Image != "jimclark106/gramin_mcp:latest" {
		t.Errorf("Expected image 'jimclark106/gramin_mcp:latest', got '%s'", catalogServer.Image)
	}

	// Verify secrets conversion (both GARMIN_EMAIL and GARMIN_PASSWORD should be secrets)
	expectedSecrets := []catalog.Secret{
		{Name: "GARMIN_EMAIL", Env: "GARMIN_EMAIL"},
		{Name: "GARMIN_PASSWORD", Env: "GARMIN_PASSWORD"},
	}
	if len(catalogServer.Secrets) != len(expectedSecrets) {
		t.Errorf("Expected %d secrets, got %d", len(expectedSecrets), len(catalogServer.Secrets))
	} else {
		for i, expected := range expectedSecrets {
			if catalogServer.Secrets[i].Name != expected.Name {
				t.Errorf("Expected secret name '%s', got '%s'", expected.Name, catalogServer.Secrets[i].Name)
			}
			if catalogServer.Secrets[i].Env != expected.Env {
				t.Errorf("Expected secret env '%s', got '%s'", expected.Env, catalogServer.Secrets[i].Env)
			}
		}
	}

	// Verify no config schemas (the environment variables are secrets, not configurable)
	if len(catalogServer.Config) != 0 {
		t.Errorf("Expected 0 config schemas, got %d", len(catalogServer.Config))
	}
}
