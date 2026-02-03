package catalog

import (
	"encoding/json"
	"testing"
)

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

	catalogJSON, err := TransformJSON(registryJSON)
	if err != nil {
		t.Fatalf("TransformJSON failed: %v", err)
	}

	var result Server
	if err := json.Unmarshal([]byte(catalogJSON), &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

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

	catalogJSON, err := TransformJSON(registryJSON)
	if err != nil {
		t.Fatalf("TransformJSON failed: %v", err)
	}

	var result Server
	if err := json.Unmarshal([]byte(catalogJSON), &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

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

	catalogJSON, err := TransformJSON(registryJSON)
	if err != nil {
		t.Fatalf("TransformJSON failed: %v", err)
	}

	var result Server
	if err := json.Unmarshal([]byte(catalogJSON), &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

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

	catalogJSON, err := TransformJSON(registryJSON)
	if err != nil {
		t.Fatalf("TransformJSON failed: %v", err)
	}

	var result Server
	if err := json.Unmarshal([]byte(catalogJSON), &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

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

	catalogJSON, err := TransformJSON(registryJSON)
	if err != nil {
		t.Fatalf("TransformJSON failed: %v", err)
	}

	var result Server
	if err := json.Unmarshal([]byte(catalogJSON), &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

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
