package catalog

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
)

// transformTestJSON is a test helper that unmarshals registry JSON, calls TransformToDocker,
// and returns both the Server and a pretty-printed JSON string of the result.
func transformTestJSON(t *testing.T, registryJSON string, resolver PyPIVersionResolver) (Server, string) {
	t.Helper()
	var serverResponse v0.ServerResponse
	if err := json.Unmarshal([]byte(registryJSON), &serverResponse); err != nil {
		t.Fatalf("Failed to parse registry JSON: %v", err)
	}
	var opts []TransformOption
	if resolver != nil {
		opts = append(opts, WithPyPIResolver(resolver))
	}
	result, err := TransformToDocker(t.Context(), serverResponse.Server, opts...)
	if err != nil {
		t.Fatalf("TransformToDocker failed: %v", err)
	}
	catalogJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal catalog JSON: %v", err)
	}
	return *result, string(catalogJSON)
}

func TestTransformOCIPackage(t *testing.T) {
	// Example with OCI package (filesystem server)
	registryJSON := `{
		"server": {
			"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
			"name": "io.github.modelcontextprotocol/filesystem",
		"title": "Filesystem MCP Server",
		"description": "Node.js server implementing Model Context Protocol (MCP) for filesystem operations",
		"version": "1.0.2",
		"packages": [
			{
				"registryType": "oci",
				"identifier": "docker.io/mcp/filesystem",
				"version": "sha256:abc123def456",
				"transport": {
					"type": "stdio"
				},
				"runtimeArguments": [
					{
						"type": "named",
						"name": "-v",
						"description": "Mount a volume into the container",
						"value": "{source_path}:{target_path}",
						"isRepeated": true,
						"variables": {
							"source_path": {
								"description": "Source path on host",
								"format": "filepath",
								"isRequired": true
							},
							"target_path": {
								"description": "Path to mount in the container",
								"isRequired": true,
								"default": "/project"
							}
						}
					},
					{
						"type": "named",
						"name": "-u",
						"value": "{uid}:{gid}",
						"variables": {
							"uid": {
								"description": "User ID",
								"default": "1000"
							},
							"gid": {
								"description": "Group ID",
								"default": "1000"
							}
						}
					}
				],
				"packageArguments": [
					{
						"type": "positional",
						"value": "/project"
					}
				],
				"environmentVariables": [
					{
						"name": "LOG_LEVEL",
						"value": "{log_level}",
						"variables": {
							"log_level": {
								"description": "Logging level (debug, info, warn, error)",
								"default": "info"
							}
						}
					}
				]
			}
		],
		"icons": [
			{
				"src": "https://example.com/filesystem-icon.png",
				"mimeType": "image/png",
				"sizes": ["48x48"]
			}
		]
		}
	}`

	result, catalogJSON := transformTestJSON(t, registryJSON, nil)

	// Verify basic fields
	if result.Name != "io-github-modelcontextprotocol-filesystem" {
		t.Errorf("Expected name 'io-github-modelcontextprotocol-filesystem', got '%s'", result.Name)
	}

	if result.Title != "Filesystem MCP Server" {
		t.Errorf("Expected title 'Filesystem MCP Server', got '%s'", result.Title)
	}

	if result.Description != "Node.js server implementing Model Context Protocol (MCP) for filesystem operations" {
		t.Errorf("Unexpected description: %s", result.Description)
	}

	// Verify OCI image
	expectedImage := "docker.io/mcp/filesystem@sha256:abc123def456"
	if result.Image != expectedImage {
		t.Errorf("Expected image '%s', got '%s'", expectedImage, result.Image)
	}

	// Verify type is "server" for OCI packages
	if result.Type != "server" {
		t.Errorf("Expected type 'server', got '%s'", result.Type)
	}

	// Verify config variables (non-secrets)
	if len(result.Config) == 0 {
		t.Error("Expected config to be present")
	} else {
		configMap, ok := result.Config[0].(map[string]any)
		if !ok {
			t.Fatal("Expected config to be a map[string]any")
		}
		properties, ok := configMap["properties"].(map[string]any)
		if !ok {
			t.Fatal("Expected properties in config")
		}
		if _, ok := properties["source_path"]; !ok {
			t.Error("Expected source_path in config properties")
		}
		if _, ok := properties["target_path"]; !ok {
			t.Error("Expected target_path in config properties")
		}
		if _, ok := properties["uid"]; !ok {
			t.Error("Expected uid in config properties")
		}
		if _, ok := properties["gid"]; !ok {
			t.Error("Expected gid in config properties")
		}
		if _, ok := properties["log_level"]; !ok {
			t.Error("Expected log_level in config properties")
		}
	}

	// Verify volumes with interpolation (includes server name)
	if len(result.Volumes) == 0 {
		t.Error("Expected volumes to be present")
	} else {
		expectedVolume := "{{io-github-modelcontextprotocol-filesystem.source_path}}:{{io-github-modelcontextprotocol-filesystem.target_path}}"
		if result.Volumes[0] != expectedVolume {
			t.Errorf("Expected volume '%s', got '%s'", expectedVolume, result.Volumes[0])
		}
	}

	// Verify user with interpolation (includes server name)
	expectedUser := "{{io-github-modelcontextprotocol-filesystem.uid}}:{{io-github-modelcontextprotocol-filesystem.gid}}"
	if result.User != expectedUser {
		t.Errorf("Expected user '%s', got '%s'", expectedUser, result.User)
	}

	// Verify command
	if len(result.Command) == 0 {
		t.Error("Expected command to be present")
	} else if result.Command[0] != "/project" {
		t.Errorf("Expected command '/project', got '%s'", result.Command[0])
	}

	// Verify environment variables (includes server name in interpolation)
	if len(result.Env) == 0 {
		t.Error("Expected environment variables to be present")
	} else {
		found := false
		for _, env := range result.Env {
			if env.Name == "LOG_LEVEL" {
				if env.Value != "{{io-github-modelcontextprotocol-filesystem.log_level}}" {
					t.Errorf("Expected LOG_LEVEL value '{{io-github-modelcontextprotocol-filesystem.log_level}}', got '%s'", env.Value)
				}
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected LOG_LEVEL environment variable")
		}
	}

	// Verify icon
	if result.Icon != "https://example.com/filesystem-icon.png" {
		t.Errorf("Expected icon URL, got '%s'", result.Icon)
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestTransformRemote(t *testing.T) {
	// Example with remote server (Google Maps Grounding Lite)
	registryJSON := `{
		"server": {
		"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
		"name": "com.google.maps/grounding-lite",
		"title": "Google Maps AI Grounding Lite",
		"version": "v0.1.0",
		"description": "Experimental MCP server providing Google Maps data with place search, weather, and routing capabilities",
		"websiteUrl": "https://developers.google.com/maps/ai/grounding-lite",
		"remotes": [
			{
				"type": "streamable-http",
				"url": "https://mapstools.googleapis.com/mcp",
				"headers": [
					{
						"name": "X-Goog-Api-Key",
						"value": "{api_key}",
						"variables": {
							"api_key": {
								"description": "Your Google Cloud API key with Maps Grounding Lite API enabled",
								"isRequired": true,
								"isSecret": true,
								"placeholder": "AIzaSyD..."
							}
						}
					},
					{
						"name": "X-Goog-User-Project",
						"value": "{project_id}",
						"variables": {
							"project_id": {
								"description": "Your Google Cloud project ID",
								"isRequired": true,
								"isSecret": false
							}
						}
					}
				]
			}
		],
		"icons": [
			{
				"src": "https://www.gstatic.com/images/branding/product/2x/maps_48dp.png",
				"mimeType": "image/png",
				"sizes": ["48x48"]
			}
		],
		"_meta": {
			"io.modelcontextprotocol.registry/official": {
				"status": "active",
				"publishedAt": "2025-12-11T00:00:00Z",
				"updatedAt": "2025-12-11T00:00:00Z",
				"isLatest": true
			}
		}
	}
		}`

	result, catalogJSON := transformTestJSON(t, registryJSON, nil)

	// Verify basic fields
	if result.Name != "com-google-maps-grounding-lite" {
		t.Errorf("Expected name 'com-google-maps-grounding-lite', got '%s'", result.Name)
	}

	if result.Title != "Google Maps AI Grounding Lite" {
		t.Errorf("Expected title 'Google Maps AI Grounding Lite', got '%s'", result.Title)
	}

	if result.Description != "Experimental MCP server providing Google Maps data with place search, weather, and routing capabilities" {
		t.Errorf("Unexpected description: %s", result.Description)
	}

	// Verify type is remote
	if result.Type != "remote" {
		t.Errorf("Expected type 'remote', got '%s'", result.Type)
	}

	// Verify remote configuration
	if result.Remote.URL == "" {
		t.Fatal("Expected remote to be present")
	}

	if result.Remote.URL != "https://mapstools.googleapis.com/mcp" {
		t.Errorf("Expected remote URL 'https://mapstools.googleapis.com/mcp', got '%s'", result.Remote.URL)
	}

	if result.Remote.Transport != "streamable-http" {
		t.Errorf("Expected transport type 'streamable-http', got '%s'", result.Remote.Transport)
	}

	// Verify headers with interpolation
	if result.Remote.Headers == nil {
		t.Fatal("Expected headers to be present")
	}

	if apiKey, ok := result.Remote.Headers["X-Goog-Api-Key"]; !ok {
		t.Error("Expected X-Goog-Api-Key header")
	} else {
		expectedKey := "${API_KEY}"
		if apiKey != expectedKey {
			t.Errorf("Expected api key interpolation '%s', got '%s'", expectedKey, apiKey)
		}
	}

	if projectID, ok := result.Remote.Headers["X-Goog-User-Project"]; !ok {
		t.Error("Expected X-Goog-User-Project header")
	} else {
		expectedProjectID := "{{com-google-maps-grounding-lite.project_id}}"
		if projectID != expectedProjectID {
			t.Errorf("Expected project_id interpolation '%s', got '%s'", expectedProjectID, projectID)
		}
	}

	// Verify secrets (api_key should be a secret)
	if len(result.Secrets) == 0 {
		t.Error("Expected secrets to be present")
	} else {
		found := false
		for _, secret := range result.Secrets {
			if secret.Name == "com-google-maps-grounding-lite.api_key" {
				if secret.Env != "API_KEY" {
					t.Errorf("Expected secret env 'API_KEY', got '%s'", secret.Env)
				}
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected api_key secret")
		}
	}

	// Verify config (project_id should be config, not secret)
	if len(result.Config) == 0 {
		t.Error("Expected config to be present")
	} else {
		configMap, ok := result.Config[0].(map[string]any)
		if !ok {
			t.Fatal("Expected config to be a map[string]any")
		}
		properties, ok := configMap["properties"].(map[string]any)
		if !ok {
			t.Fatal("Expected properties in config")
		}
		if prop, ok := properties["project_id"]; !ok {
			t.Error("Expected project_id in config properties")
		} else {
			propMap, ok := prop.(map[string]any)
			if !ok {
				t.Fatal("Expected property to be a map[string]any")
			}
			if propMap["type"] != "string" {
				t.Errorf("Expected project_id type 'string', got '%v'", propMap["type"])
			}
		}
	}

	// Verify icon
	if result.Icon != "https://www.gstatic.com/images/branding/product/2x/maps_48dp.png" {
		t.Errorf("Expected icon URL, got '%s'", result.Icon)
	}

	// No image should be present for remote servers
	if result.Image != "" {
		t.Errorf("Expected no image for remote server, got '%s'", result.Image)
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestTransformRemoteWithOAuth(t *testing.T) {
	// Example with OAuth (GKE server)
	registryJSON := `{
		"server": {
		"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
		"name": "com.googleapis.container/gke",
		"title": "Google Kubernetes Engine (GKE) MCP Server",
		"description": "Manage GKE clusters and Kubernetes resources through MCP",
		"version": "1.0.0",
		"websiteUrl": "https://cloud.google.com/kubernetes-engine/docs/reference/mcp",
		"remotes": [
			{
				"type": "streamable-http",
				"url": "https://container.googleapis.com/mcp",
				"headers": [
					{
						"name": "x-goog-user-project",
						"value": "{project_id}",
						"variables": {
							"project_id": {
								"description": "Your Google project id",
								"isRequired": true,
								"isSecret": true,
								"placeholder": "project-1234..."
							}
						}
					}
				]
			}
		],
		"_meta": {
			"io.modelcontextprotocol.registry/publisher-provided": {
				"oauth": {
					"providers": [
						{
							"provider": "google",
							"secret": "google.access_token",
							"env": "ACCESS_TOKEN"
						}
					],
					"scopes": []
				}
			}
		}
	}
		}`

	result, catalogJSON := transformTestJSON(t, registryJSON, nil)

	// Verify OAuth is present
	if result.OAuth == nil {
		t.Fatal("Expected OAuth to be present")
	}

	// Verify OAuth structure
	if len(result.OAuth.Providers) == 0 {
		t.Error("Expected at least one OAuth provider")
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestTransformSimpleRemote(t *testing.T) {
	// Example with simple remote (no headers, no variables)
	registryJSON := `{
		"server": {
		"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
		"name": "com.docker/grafana-internal",
		"title": "Docker Internal Grafana Server",
		"description": "Internal Grafana MCP server. Only accessible to Docker employees",
		"version": "0.1.0",
		"websiteUrl": "https://www.notion.so/dockerinc/Grafana-MCP",
		"remotes": [
			{
				"type": "streamable-http",
				"url": "https://mcp-grafana.s.us-east-1.aws.dckr.io/mcp"
			}
		]
	}
		}`

	result, catalogJSON := transformTestJSON(t, registryJSON, nil)

	// Verify basic fields
	if result.Name != "com-docker-grafana-internal" {
		t.Errorf("Expected name 'com-docker-grafana-internal', got '%s'", result.Name)
	}

	if result.Type != "remote" {
		t.Errorf("Expected type 'remote', got '%s'", result.Type)
	}

	// Verify remote
	if result.Remote.URL == "" {
		t.Fatal("Expected remote to be present")
	}

	if result.Remote.URL != "https://mcp-grafana.s.us-east-1.aws.dckr.io/mcp" {
		t.Errorf("Unexpected remote URL: %s", result.Remote.URL)
	}

	// No headers should be present
	if len(result.Remote.Headers) > 0 {
		t.Error("Expected no headers for simple remote")
	}

	// No config or secrets should be present
	if len(result.Config) > 0 {
		t.Error("Expected no config for simple remote")
	}

	if len(result.Secrets) > 0 {
		t.Error("Expected no secrets for simple remote")
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestTransformOCIWithDirectSecrets(t *testing.T) {
	// Example with OCI package and direct secret environment variables (Garmin MCP)
	registryJSON := `{
		"server": {
		"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
		"name": "io.github.slimslenderslacks/garmin_mcp",
		"description": "exposes your fitness and health data to Claude and other MCP-compatible clients",
		"status": "active",
		"repository": {
			"url": "https://github.com/slimslenderslacks/poci",
			"source": "github"
		},
		"version": "0.1.1",
		"packages": [
			{
				"registryType": "oci",
				"registryBaseUrl": "https://docker.io",
				"identifier": "jimclark106/gramin_mcp",
				"version": "sha256:637379b17fc12103bb00a52ccf27368208fd8009e6efe2272b623b1a5431814a",
				"transport": {
					"type": "stdio"
				},
				"environmentVariables": [
					{
						"name": "GARMIN_EMAIL",
						"description": "Garmin Connect email address",
						"isRequired": true,
						"isSecret": true
					},
					{
						"name": "GARMIN_PASSWORD",
						"description": "Garmin Connect password",
						"isRequired": true,
						"isSecret": true
					}
				]
			}
		]
	}
		}`

	result, catalogJSON := transformTestJSON(t, registryJSON, nil)

	// Verify basic fields
	if result.Name != "io-github-slimslenderslacks-garmin_mcp" {
		t.Errorf("Expected name 'io-github-slimslenderslacks-garmin_mcp', got '%s'", result.Name)
	}

	if result.Description != "exposes your fitness and health data to Claude and other MCP-compatible clients" {
		t.Errorf("Unexpected description: %s", result.Description)
	}

	// Verify OCI image
	expectedImage := "jimclark106/gramin_mcp@sha256:637379b17fc12103bb00a52ccf27368208fd8009e6efe2272b623b1a5431814a"
	if result.Image != expectedImage {
		t.Errorf("Expected image '%s', got '%s'", expectedImage, result.Image)
	}

	// Verify type is "server" for OCI packages
	if result.Type != "server" {
		t.Errorf("Expected type 'server', got '%s'", result.Type)
	}

	// Verify secrets - both GARMIN_EMAIL and GARMIN_PASSWORD should be secrets
	if len(result.Secrets) != 2 {
		t.Errorf("Expected 2 secrets, got %d", len(result.Secrets))
	} else {
		// Check for GARMIN_EMAIL secret
		foundEmail := false
		foundPassword := false
		for _, secret := range result.Secrets {
			if secret.Name == "io-github-slimslenderslacks-garmin_mcp.GARMIN_EMAIL" {
				if secret.Env != "GARMIN_EMAIL" {
					t.Errorf("Expected secret env 'GARMIN_EMAIL', got '%s'", secret.Env)
				}
				foundEmail = true
			}
			if secret.Name == "io-github-slimslenderslacks-garmin_mcp.GARMIN_PASSWORD" {
				if secret.Env != "GARMIN_PASSWORD" {
					t.Errorf("Expected secret env 'GARMIN_PASSWORD', got '%s'", secret.Env)
				}
				foundPassword = true
			}
		}
		if !foundEmail {
			t.Error("Expected GARMIN_EMAIL secret")
		}
		if !foundPassword {
			t.Error("Expected GARMIN_PASSWORD secret")
		}
	}

	// Verify no environment variables (secrets should only be in secrets array, not env)
	if len(result.Env) > 0 {
		t.Errorf("Expected no environment variables (secrets should only be in secrets array), got %d", len(result.Env))
	}

	// No config should be present since all variables are secrets
	if len(result.Config) > 0 {
		t.Error("Expected no config for server with only secrets")
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestTransformPyPI(t *testing.T) {
	// Example with basic PyPI package
	registryJSON := `{
		"server": {
			"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
			"name": "io.github.stevenvo/slack-mcp-server",
			"title": "Slack MCP Server",
			"description": "MCP server for Slack integration",
			"version": "0.1.0",
			"packages": [{
				"registryType": "pypi",
				"registryBaseUrl": "https://pypi.org",
				"identifier": "slack-mcp-server-v2",
				"version": "0.1.0",
				"transport": {"type": "stdio"},
				"environmentVariables": [
					{
						"name": "SLACK_USER_TOKEN",
						"description": "Slack user token for authentication",
						"isSecret": true,
						"isRequired": true
					}
				]
			}]
		}
	}`

	// Mock resolver that returns Python 3.12 (simulates ==3.12 or ~=3.12)
	mockResolver := func(_ context.Context, _, _, _ string) (string, bool) {
		return "3.12", true
	}

	result, catalogJSON := transformTestJSON(t, registryJSON, mockResolver)

	// Verify basic fields
	if result.Name != "io-github-stevenvo-slack-mcp-server" {
		t.Errorf("Expected name 'io-github-stevenvo-slack-mcp-server', got '%s'", result.Name)
	}

	if result.Title != "Slack MCP Server" {
		t.Errorf("Expected title 'Slack MCP Server', got '%s'", result.Title)
	}

	// Verify type is "server" (PyPI packages are treated as regular servers)
	if result.Type != "server" {
		t.Errorf("Expected type 'server', got '%s'", result.Type)
	}

	// Verify image is the uv Docker image
	expectedImage := "ghcr.io/astral-sh/uv:python3.12-bookworm-slim"
	if result.Image != expectedImage {
		t.Errorf("Expected image '%s', got '%s'", expectedImage, result.Image)
	}

	// Verify command
	expectedCommand := []string{"uvx", "--from", "slack-mcp-server-v2==0.1.0", "slack-mcp-server-v2"}
	if len(result.Command) != len(expectedCommand) {
		t.Errorf("Expected command length %d, got %d", len(expectedCommand), len(result.Command))
	} else {
		for i, cmd := range expectedCommand {
			if result.Command[i] != cmd {
				t.Errorf("Expected command[%d] '%s', got '%s'", i, cmd, result.Command[i])
			}
		}
	}

	// Verify PyPI servers are long-lived
	if !result.LongLived {
		t.Error("Expected PyPI server to be long-lived")
	}

	// Verify cache volume
	if len(result.Volumes) == 0 {
		t.Error("Expected volumes to be present")
	} else {
		expectedVolume := "docker-mcp-uv-cache-io-github-stevenvo-slack-mcp-server:/root/.cache/uv"
		if result.Volumes[0] != expectedVolume {
			t.Errorf("Expected volume '%s', got '%s'", expectedVolume, result.Volumes[0])
		}
	}

	// Verify secrets
	if len(result.Secrets) == 0 {
		t.Error("Expected secrets to be present")
	} else {
		found := false
		for _, secret := range result.Secrets {
			if secret.Name == "io-github-stevenvo-slack-mcp-server.SLACK_USER_TOKEN" {
				if secret.Env != "SLACK_USER_TOKEN" {
					t.Errorf("Expected secret env 'SLACK_USER_TOKEN', got '%s'", secret.Env)
				}
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected SLACK_USER_TOKEN secret")
		}
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestTransformPyPIWithCustomRegistry(t *testing.T) {
	// Example with custom PyPI registry
	registryJSON := `{
		"server": {
			"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
			"name": "com.example/custom-pypi-server",
			"title": "Custom PyPI Server",
			"description": "MCP server from custom PyPI registry",
			"version": "1.0.0",
			"packages": [{
				"registryType": "pypi",
				"registryBaseUrl": "https://custom.pypi.org",
				"identifier": "my-custom-package",
				"version": "1.0.0",
				"transport": {"type": "stdio"}
			}]
		}
	}`

	result, catalogJSON := transformTestJSON(t, registryJSON, nil)

	// Verify type is "server" (PyPI packages are treated as regular servers)
	if result.Type != "server" {
		t.Errorf("Expected type 'server', got '%s'", result.Type)
	}

	// Verify command includes --index-url
	expectedCommand := []string{"uvx", "--index-url", "https://custom.pypi.org", "--from", "my-custom-package==1.0.0", "my-custom-package"}
	if len(result.Command) != len(expectedCommand) {
		t.Errorf("Expected command length %d, got %d", len(expectedCommand), len(result.Command))
	} else {
		for i, cmd := range expectedCommand {
			if result.Command[i] != cmd {
				t.Errorf("Expected command[%d] '%s', got '%s'", i, cmd, result.Command[i])
			}
		}
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestTransformPyPIWithPackageArgs(t *testing.T) {
	// Example with PyPI package and package arguments
	registryJSON := `{
		"server": {
			"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
			"name": "io.example/pypi-with-args",
			"title": "PyPI Server with Arguments",
			"description": "MCP server with command-line arguments",
			"version": "1.0.0",
			"packages": [{
				"registryType": "pypi",
				"identifier": "my-server",
				"version": "1.0.0",
				"transport": {"type": "stdio"},
				"packageArguments": [
					{
						"type": "positional",
						"value": "--verbose"
					},
					{
						"type": "positional",
						"value": "/data"
					}
				]
			}]
		}
	}`

	result, catalogJSON := transformTestJSON(t, registryJSON, nil)

	// Verify type is "server" (PyPI packages are treated as regular servers)
	if result.Type != "server" {
		t.Errorf("Expected type 'server', got '%s'", result.Type)
	}

	// Verify command includes package arguments appended
	expectedCommand := []string{"uvx", "--from", "my-server==1.0.0", "my-server", "--verbose", "/data"}
	if len(result.Command) != len(expectedCommand) {
		t.Errorf("Expected command length %d, got %d", len(expectedCommand), len(result.Command))
	} else {
		for i, cmd := range expectedCommand {
			if result.Command[i] != cmd {
				t.Errorf("Expected command[%d] '%s', got '%s'", i, cmd, result.Command[i])
			}
		}
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestTransformPyPIWithoutVersion(t *testing.T) {
	// Example with PyPI package without version (should run latest)
	registryJSON := `{
		"server": {
			"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
			"name": "io.example/pypi-no-version",
			"title": "PyPI Server No Version",
			"description": "MCP server without version specified",
			"version": "1.0.0",
			"packages": [{
				"registryType": "pypi",
				"identifier": "my-latest-server",
				"transport": {"type": "stdio"}
			}]
		}
	}`

	result, catalogJSON := transformTestJSON(t, registryJSON, nil)

	// Verify type is "server" (PyPI packages are treated as regular servers)
	if result.Type != "server" {
		t.Errorf("Expected type 'server', got '%s'", result.Type)
	}

	// Verify command does NOT include --from (will run latest version)
	expectedCommand := []string{"uvx", "my-latest-server"}
	if len(result.Command) != len(expectedCommand) {
		t.Errorf("Expected command length %d, got %d", len(expectedCommand), len(result.Command))
	} else {
		for i, cmd := range expectedCommand {
			if result.Command[i] != cmd {
				t.Errorf("Expected command[%d] '%s', got '%s'", i, cmd, result.Command[i])
			}
		}
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestTransformPyPIWithEnvVariables(t *testing.T) {
	// Example with PyPI package with both secrets and config env vars
	registryJSON := `{
		"server": {
			"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
			"name": "io.example/pypi-with-env",
			"title": "PyPI Server with Environment Variables",
			"description": "MCP server with environment variables",
			"version": "1.0.0",
			"packages": [{
				"registryType": "pypi",
				"identifier": "my-env-server",
				"version": "1.0.0",
				"transport": {"type": "stdio"},
				"environmentVariables": [
					{
						"name": "API_KEY",
						"description": "API key for authentication",
						"isSecret": true,
						"isRequired": true
					},
					{
						"name": "LOG_LEVEL",
						"value": "{log_level}",
						"variables": {
							"log_level": {
								"description": "Logging level",
								"default": "info"
							}
						}
					}
				]
			}]
		}
	}`

	result, catalogJSON := transformTestJSON(t, registryJSON, nil)

	// Verify type is "server" (PyPI packages are treated as regular servers)
	if result.Type != "server" {
		t.Errorf("Expected type 'server', got '%s'", result.Type)
	}

	// Verify secrets
	if len(result.Secrets) == 0 {
		t.Error("Expected secrets to be present")
	} else {
		found := false
		for _, secret := range result.Secrets {
			if secret.Name == "io-example-pypi-with-env.API_KEY" {
				if secret.Env != "API_KEY" {
					t.Errorf("Expected secret env 'API_KEY', got '%s'", secret.Env)
				}
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected API_KEY secret")
		}
	}

	// Verify environment variables
	if len(result.Env) == 0 {
		t.Error("Expected environment variables to be present")
	} else {
		found := false
		for _, env := range result.Env {
			if env.Name == "LOG_LEVEL" {
				if env.Value != "{{io-example-pypi-with-env.log_level}}" {
					t.Errorf("Expected LOG_LEVEL value '{{io-example-pypi-with-env.log_level}}', got '%s'", env.Value)
				}
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected LOG_LEVEL environment variable")
		}
	}

	// Verify config
	if len(result.Config) == 0 {
		t.Error("Expected config to be present")
	} else {
		configMap, ok := result.Config[0].(map[string]any)
		if !ok {
			t.Fatal("Expected config to be a map[string]any")
		}
		properties, ok := configMap["properties"].(map[string]any)
		if !ok {
			t.Fatal("Expected properties in config")
		}
		if _, ok := properties["log_level"]; !ok {
			t.Error("Expected log_level in config properties")
		}
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestBuildConfigSchema_NoRequiredFields(t *testing.T) {
	// Bug case: config vars exist but none are required.
	// "required" key must be omitted, not serialized as null.
	configVars := map[string]model.Input{
		"log_level": {
			Description: "Logging level",
			Default:     "info",
		},
		"timeout": {
			Description: "Request timeout",
			Default:     "30",
		},
	}

	result := buildConfigSchema(configVars, "test-server")
	if len(result) != 1 {
		t.Fatalf("Expected 1 config entry, got %d", len(result))
	}

	configMap, ok := result[0].(map[string]any)
	if !ok {
		t.Fatal("Expected config entry to be map[string]any")
	}

	// The "required" key must not exist in the map
	if _, exists := configMap["required"]; exists {
		t.Fatal("Expected 'required' key to be absent when no fields are required")
	}

	// Verify it marshals cleanly (no "required": null)
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	jsonStr := string(jsonBytes)
	if strings.Contains(jsonStr, `"required"`) {
		t.Errorf("JSON should not contain 'required' key, got: %s", jsonStr)
	}
}

func TestBuildConfigSchema_WithRequiredFields(t *testing.T) {
	configVars := map[string]model.Input{
		"api_endpoint": {
			Description: "API endpoint URL",
			IsRequired:  true,
		},
		"log_level": {
			Description: "Logging level",
			Default:     "info",
		},
	}

	result := buildConfigSchema(configVars, "test-server")
	if len(result) != 1 {
		t.Fatalf("Expected 1 config entry, got %d", len(result))
	}

	configMap, ok := result[0].(map[string]any)
	if !ok {
		t.Fatal("Expected config entry to be map[string]any")
	}

	required, exists := configMap["required"]
	if !exists {
		t.Fatal("Expected 'required' key to be present")
	}

	requiredSlice, ok := required.([]string)
	if !ok {
		t.Fatalf("Expected required to be []string, got %T", required)
	}

	if len(requiredSlice) != 1 || requiredSlice[0] != "api_endpoint" {
		t.Errorf("Expected required to be [api_endpoint], got %v", requiredSlice)
	}
}

func TestBuildConfigSchema_Empty(t *testing.T) {
	result := buildConfigSchema(map[string]model.Input{}, "test-server")
	if result != nil {
		t.Errorf("Expected nil for empty config vars, got %v", result)
	}
}

func TestTransformOCIWithOptionalConfig(t *testing.T) {
	// Regression test: servers with config vars where none are required
	// must not produce "required": null in the JSON output.
	registryJSON := `{
		"server": {
			"$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
			"name": "io.example/optional-config-server",
			"title": "Optional Config Server",
			"description": "Server with only optional config variables",
			"version": "1.0.0",
			"packages": [
				{
					"registryType": "oci",
					"identifier": "docker.io/example/server",
					"version": "sha256:abc123",
					"transport": { "type": "stdio" },
					"environmentVariables": [
						{
							"name": "LOG_LEVEL",
							"value": "{log_level}",
							"variables": {
								"log_level": {
									"description": "Logging level",
									"default": "info"
								}
							}
						},
						{
							"name": "TIMEOUT",
							"value": "{timeout}",
							"variables": {
								"timeout": {
									"description": "Request timeout in seconds",
									"format": "number",
									"default": "30"
								}
							}
						}
					]
				}
			]
		}
	}`

	result, catalogJSON := transformTestJSON(t, registryJSON, nil)

	// The critical assertion: JSON must not contain "required": null
	if strings.Contains(catalogJSON, `"required": null`) || strings.Contains(catalogJSON, `"required":null`) {
		t.Errorf("Config schema must not contain 'required: null', got:\n%s", catalogJSON)
	}

	// Verify config exists with properties but no required field
	if len(result.Config) == 0 {
		t.Fatal("Expected config to be present")
	}

	configMap, ok := result.Config[0].(map[string]any)
	if !ok {
		t.Fatal("Expected config to be a map[string]any")
	}

	if _, exists := configMap["required"]; exists {
		t.Error("Expected 'required' key to be absent when no config vars are required")
	}

	// Verify properties are still present
	properties, ok := configMap["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties in config")
	}
	if _, ok := properties["log_level"]; !ok {
		t.Error("Expected log_level in config properties")
	}
	if _, ok := properties["timeout"]; !ok {
		t.Error("Expected timeout in config properties")
	}

	t.Logf("Catalog JSON:\n%s", catalogJSON)
}

func TestTransformPyPIDisallowed(t *testing.T) {
	// PyPI-only server should be rejected when WithAllowPyPI(false) is set
	registryJSON := `{
		"server": {
			"name": "io.github.example/pypi-only-server",
			"title": "PyPI Only Server",
			"description": "Server with only PyPI package",
			"version": "1.0.0",
			"packages": [{
				"registryType": "pypi",
				"registryBaseUrl": "https://pypi.org",
				"identifier": "example-mcp-server",
				"version": "1.0.0",
				"transport": {"type": "stdio"}
			}]
		}
	}`

	var serverResponse v0.ServerResponse
	if err := json.Unmarshal([]byte(registryJSON), &serverResponse); err != nil {
		t.Fatalf("Failed to parse registry JSON: %v", err)
	}

	_, err := TransformToDocker(t.Context(), serverResponse.Server, WithAllowPyPI(false))
	if err == nil {
		t.Fatal("Expected error when PyPI is disallowed, got nil")
	}
	if !strings.Contains(err.Error(), "incompatible server") {
		t.Errorf("Expected incompatible server error, got: %v", err)
	}
}

func TestTransformPyPIAllowedByDefault(t *testing.T) {
	// PyPI-only server should work with default options (no WithAllowPyPI)
	registryJSON := `{
		"server": {
			"name": "io.github.example/pypi-default",
			"title": "PyPI Default Server",
			"description": "Server with only PyPI package",
			"version": "1.0.0",
			"packages": [{
				"registryType": "pypi",
				"registryBaseUrl": "https://pypi.org",
				"identifier": "example-mcp-server",
				"version": "1.0.0",
				"transport": {"type": "stdio"}
			}]
		}
	}`

	var serverResponse v0.ServerResponse
	if err := json.Unmarshal([]byte(registryJSON), &serverResponse); err != nil {
		t.Fatalf("Failed to parse registry JSON: %v", err)
	}

	result, err := TransformToDocker(t.Context(), serverResponse.Server)
	if err != nil {
		t.Fatalf("Expected success for PyPI with default options, got: %v", err)
	}
	if result.Type != "server" {
		t.Errorf("Expected type 'server', got '%s'", result.Type)
	}
	if result.Image == "" {
		t.Error("Expected image to be set for PyPI server")
	}
}

func TestTransformPyPIPackageNotFound(t *testing.T) {
	registryJSON := `{
		"server": {
			"name": "io.github.example/pypi-not-found",
			"title": "PyPI Not Found Server",
			"description": "Server whose PyPI package does not exist",
			"version": "1.0.0",
			"packages": [{
				"registryType": "pypi",
				"registryBaseUrl": "https://pypi.org",
				"identifier": "nonexistent-mcp-server",
				"version": "9.9.9",
				"transport": {"type": "stdio"}
			}]
		}
	}`

	notFoundResolver := func(_ context.Context, _, _, _ string) (string, bool) {
		return "", false
	}

	var serverResponse v0.ServerResponse
	if err := json.Unmarshal([]byte(registryJSON), &serverResponse); err != nil {
		t.Fatalf("Failed to parse registry JSON: %v", err)
	}

	_, err := TransformToDocker(t.Context(), serverResponse.Server, WithPyPIResolver(notFoundResolver))
	if err == nil {
		t.Fatal("Expected error when PyPI package is not found, got nil")
	}
	if !strings.Contains(err.Error(), "was not found") {
		t.Errorf("Expected 'was not found' in error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "nonexistent-mcp-server@9.9.9") {
		t.Errorf("Expected package identifier and version in error message, got: %v", err)
	}
}
