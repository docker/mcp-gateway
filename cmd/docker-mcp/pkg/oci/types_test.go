package oci

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/catalog"
)

func TestServerDetailParsing(t *testing.T) {
	// Read test data from external JSON file
	testDataPath := filepath.Join("..", "..", "..", "..", "test", "testdata", "mcpregistry", "server_garmin_mcp.json")
	jsonData, err := os.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("Failed to read test data file %s: %v", testDataPath, err)
	}

	var serverDetail ServerDetail
	err = json.Unmarshal(jsonData, &serverDetail)
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

	// Verify version (direct field, not VersionDetail struct in this test data)
	if serverDetail.Version != "0.1.1" {
		t.Errorf("Expected version '0.1.1', got '%s'", serverDetail.Version)
	}

	// Verify repository
	if serverDetail.Repository.URL == "" {
		t.Error("Expected Repository URL to be non-empty")
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

	// Note: Meta field is not present in this test data file, which is expected
}

func TestServerDetailToCatalogServer(t *testing.T) {
	// Read test data from external JSON file
	testDataPath := filepath.Join("..", "..", "..", "..", "test", "testdata", "mcpregistry", "server_garmin_mcp.json")
	jsonData, err := os.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("Failed to read test data file %s: %v", testDataPath, err)
	}

	var serverDetail ServerDetail
	err = json.Unmarshal(jsonData, &serverDetail)
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
		{Name: "io_github_slimslenderslacks/garmin_mcp.GARMIN_EMAIL", Env: "GARMIN_EMAIL"},
		{Name: "io_github_slimslenderslacks/garmin_mcp.GARMIN_PASSWORD", Env: "GARMIN_PASSWORD"},
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

func TestConversionForFileSystem(t *testing.T) {
	// Read test data from external JSON file
	testDataPath := filepath.Join("..", "..", "..", "..", "test", "testdata", "mcpregistry", "server_filesystem.json")
	jsonData, err := os.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("Failed to read test data file %s: %v", testDataPath, err)
	}

	var serverDetail ServerDetail
	err = json.Unmarshal(jsonData, &serverDetail)
	if err != nil {
		t.Fatalf("Failed to parse filesystem JSON: %v", err)
	}

	// Verify basic filesystem server fields
	if serverDetail.Name != "io.github.slimslenderslacks/filesystem" {
		t.Errorf("Expected name 'io.github.slimslenderslacks/filesystem', got '%s'", serverDetail.Name)
	}

	if serverDetail.Description != "Node.js server implementing Model Context Protocol (MCP) for filesystem operations." {
		t.Errorf("Expected filesystem description to match, got '%s'", serverDetail.Description)
	}

	if serverDetail.Version != "1.0.2" {
		t.Errorf("Expected version '1.0.2', got '%s'", serverDetail.Version)
	}

	// Verify repository
	if serverDetail.Repository.URL != "https://github.com/modelcontextprotocol/servers" {
		t.Errorf("Expected repository URL 'https://github.com/modelcontextprotocol/servers', got '%s'", serverDetail.Repository.URL)
	}
	if serverDetail.Repository.Source != "github" {
		t.Errorf("Expected repository source 'github', got '%s'", serverDetail.Repository.Source)
	}

	// Verify packages
	if len(serverDetail.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(serverDetail.Packages))
		return
	}

	pkg := serverDetail.Packages[0]
	if pkg.RegistryType != "oci" {
		t.Errorf("Expected registry type 'oci', got '%s'", pkg.RegistryType)
	}
	if pkg.Identifier != "mcp/filesystem" {
		t.Errorf("Expected identifier 'mcp/filesystem', got '%s'", pkg.Identifier)
	}
	if pkg.Version != "1.0.2" {
		t.Errorf("Expected version '1.0.2', got '%s'", pkg.Version)
	}

	// Verify runtime arguments
	if len(pkg.RuntimeOptions) != 1 {
		t.Errorf("Expected 1 runtime argument, got %d", len(pkg.RuntimeOptions))
		return
	}

	runtimeArg := pkg.RuntimeOptions[0]
	if runtimeArg.Type != "named" {
		t.Errorf("Expected runtime argument type 'named', got '%s'", runtimeArg.Type)
	}
	if runtimeArg.Name != "--mount" {
		t.Errorf("Expected runtime argument name '--mount', got '%s'", runtimeArg.Name)
	}
	if runtimeArg.Value != "type=bind,src={source_path},dst={target_path}" {
		t.Errorf("Expected runtime argument value 'type=bind,src={source_path},dst={target_path}', got '%s'", runtimeArg.Value)
	}
	if !runtimeArg.Required {
		t.Error("Expected runtime argument to be required")
	}
	if !runtimeArg.IsRepeated {
		t.Error("Expected runtime argument to be repeatable")
	}

	// Verify runtime argument variables
	if len(runtimeArg.Variables) != 2 {
		t.Errorf("Expected 2 runtime argument variables, got %d", len(runtimeArg.Variables))
		return
	}

	if sourcePath, exists := runtimeArg.Variables["source_path"]; !exists {
		t.Error("Expected 'source_path' variable to exist")
	} else {
		if sourcePath.Description != "Source path on host" {
			t.Errorf("Expected source_path description 'Source path on host', got '%s'", sourcePath.Description)
		}
		if sourcePath.Format != "filepath" {
			t.Errorf("Expected source_path format 'filepath', got '%s'", sourcePath.Format)
		}
		if !sourcePath.Required {
			t.Error("Expected source_path to be required")
		}
	}

	if targetPath, exists := runtimeArg.Variables["target_path"]; !exists {
		t.Error("Expected 'target_path' variable to exist")
	} else {
		if targetPath.Description != "Path to mount in the container. It should be rooted in `/project` directory." {
			t.Errorf("Expected target_path description to match, got '%s'", targetPath.Description)
		}
		if targetPath.DefaultValue != "/project" {
			t.Errorf("Expected target_path default '/project', got '%v'", targetPath.DefaultValue)
		}
		if !targetPath.Required {
			t.Error("Expected target_path to be required")
		}
	}

	// Verify package arguments
	if len(pkg.PackageArguments) != 1 {
		t.Errorf("Expected 1 package argument, got %d", len(pkg.PackageArguments))
		return
	}

	packageArg := pkg.PackageArguments[0]
	if packageArg.Type != "positional" {
		t.Errorf("Expected package argument type 'positional', got '%s'", packageArg.Type)
	}
	if packageArg.Value != "/project" {
		t.Errorf("Expected package argument value '/project', got '%s'", packageArg.Value)
	}
	if packageArg.ValueHint != "target_dir" {
		t.Errorf("Expected package argument value hint 'target_dir', got '%s'", packageArg.ValueHint)
	}

	// Verify environment variables
	if len(pkg.Env) != 1 {
		t.Errorf("Expected 1 environment variable, got %d", len(pkg.Env))
		return
	}

	env := pkg.Env[0]
	if env.Name != "LOG_LEVEL" {
		t.Errorf("Expected env var name 'LOG_LEVEL', got '%s'", env.Name)
	}
	if env.Description != "Logging level (debug, info, warn, error)" {
		t.Errorf("Expected env var description 'Logging level (debug, info, warn, error)', got '%s'", env.Description)
	}
	if env.DefaultValue != "info" {
		t.Errorf("Expected env var default 'info', got '%v'", env.DefaultValue)
	}
	if env.Required {
		t.Error("Expected env var to not be required (has default)")
	}
	if env.Secret {
		t.Error("Expected env var to not be secret")
	}

	// Test conversion to catalog server
	catalogServer := serverDetail.ToCatalogServer()

	// Verify basic conversion
	if catalogServer.Description != "Node.js server implementing Model Context Protocol (MCP) for filesystem operations." {
		t.Errorf("Expected catalog server description to match, got '%s'", catalogServer.Description)
	}

	if catalogServer.Image != "mcp/filesystem:1.0.2" {
		t.Errorf("Expected catalog server image 'mcp/filesystem:1.0.2', got '%s'", catalogServer.Image)
	}

	// Verify no secrets (LOG_LEVEL is not secret)
	if len(catalogServer.Secrets) != 0 {
		t.Errorf("Expected 0 secrets, got %d", len(catalogServer.Secrets))
	}

	// Verify environment variables conversion (non-secret env vars should be preserved)
	expectedEnvVars := []catalog.Env{
		{Name: "LOG_LEVEL", Value: "info"}, // Default value should be used
	}
	if len(catalogServer.Env) != len(expectedEnvVars) {
		t.Errorf("Expected %d environment variables, got %d", len(expectedEnvVars), len(catalogServer.Env))
	} else {
		for i, expected := range expectedEnvVars {
			if catalogServer.Env[i].Name != expected.Name {
				t.Errorf("Expected env var name '%s', got '%s'", expected.Name, catalogServer.Env[i].Name)
			}
			if catalogServer.Env[i].Value != expected.Value {
				t.Errorf("Expected env var value '%s', got '%s'", expected.Value, catalogServer.Env[i].Value)
			}
		}
	}
}

func TestBasicServerConversion(t *testing.T) {
	// Read test data from the basic test JSON file
	testDataPath := filepath.Join("..", "..", "..", "..", "test", "testdata", "mcpregistry", "server.test.json")
	jsonData, err := os.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("Failed to read test data file %s: %v", testDataPath, err)
	}

	var serverDetail ServerDetail
	err = json.Unmarshal(jsonData, &serverDetail)
	if err != nil {
		t.Fatalf("Failed to parse basic server JSON: %v", err)
	}

	// Verify basic fields
	if serverDetail.Name != "io.github.slimslenderslacks/poci" {
		t.Errorf("Expected name 'io.github.slimslenderslacks/poci', got '%s'", serverDetail.Name)
	}

	if serverDetail.Description != "construct new tools out of existing images" {
		t.Errorf("Expected description 'construct new tools out of existing images', got '%s'", serverDetail.Description)
	}

	if serverDetail.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", serverDetail.Status)
	}

	if serverDetail.Version != "1.0.12" {
		t.Errorf("Expected version '1.0.12', got '%s'", serverDetail.Version)
	}

	// Verify repository
	if serverDetail.Repository.URL != "https://github.com/slimslenderslacks/poci" {
		t.Errorf("Expected repository URL 'https://github.com/slimslenderslacks/poci', got '%s'", serverDetail.Repository.URL)
	}
	if serverDetail.Repository.Source != "github" {
		t.Errorf("Expected repository source 'github', got '%s'", serverDetail.Repository.Source)
	}

	// Verify packages
	if len(serverDetail.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(serverDetail.Packages))
		return
	}

	pkg := serverDetail.Packages[0]
	if pkg.RegistryType != "oci" {
		t.Errorf("Expected registry type 'oci', got '%s'", pkg.RegistryType)
	}
	if pkg.Identifier != "jimclark106/poci" {
		t.Errorf("Expected identifier 'jimclark106/poci', got '%s'", pkg.Identifier)
	}
	if pkg.Version != "latest" {
		t.Errorf("Expected version 'latest', got '%s'", pkg.Version)
	}

	// This basic server has no environment variables or runtime arguments
	if len(pkg.Env) != 0 {
		t.Errorf("Expected 0 environment variables, got %d", len(pkg.Env))
	}
	if len(pkg.RuntimeOptions) != 0 {
		t.Errorf("Expected 0 runtime arguments, got %d", len(pkg.RuntimeOptions))
	}

	// Test conversion to catalog server
	catalogServer := serverDetail.ToCatalogServer()

	// Verify basic conversion
	if catalogServer.Description != "construct new tools out of existing images" {
		t.Errorf("Expected catalog server description to match, got '%s'", catalogServer.Description)
	}

	if catalogServer.Image != "jimclark106/poci:latest" {
		t.Errorf("Expected catalog server image 'jimclark106/poci:latest', got '%s'", catalogServer.Image)
	}

	// Verify no secrets, environment variables, or configuration
	if len(catalogServer.Secrets) != 0 {
		t.Errorf("Expected 0 secrets, got %d", len(catalogServer.Secrets))
	}
	if len(catalogServer.Env) != 0 {
		t.Errorf("Expected 0 environment variables, got %d", len(catalogServer.Env))
	}
	if len(catalogServer.Config) != 0 {
		t.Errorf("Expected 0 config schemas, got %d", len(catalogServer.Config))
	}
}

func TestRemoteServerConversion(t *testing.T) {
	// Read test data from remote server JSON file
	testDataPath := filepath.Join("..", "..", "..", "..", "test", "testdata", "mcpregistry", "server.remote.json")
	jsonData, err := os.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("Failed to read test data file %s: %v", testDataPath, err)
	}

	var serverDetail ServerDetail
	err = json.Unmarshal(jsonData, &serverDetail)
	if err != nil {
		t.Fatalf("Failed to parse remote server JSON: %v", err)
	}

	// Verify basic fields
	if serverDetail.Name != "io.github.slimslenderslacks/remote" {
		t.Errorf("Expected name 'io.github.slimslenderslacks/remote', got '%s'", serverDetail.Name)
	}

	if serverDetail.Description != "remote example" {
		t.Errorf("Expected description 'remote example', got '%s'", serverDetail.Description)
	}

	if serverDetail.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", serverDetail.Status)
	}

	if serverDetail.Version != "0.1.0" {
		t.Errorf("Expected version '0.1.0', got '%s'", serverDetail.Version)
	}

	// Verify repository
	if serverDetail.Repository.URL != "https://github.com/slimslenderslacks/poci" {
		t.Errorf("Expected repository URL 'https://github.com/slimslenderslacks/poci', got '%s'", serverDetail.Repository.URL)
	}
	if serverDetail.Repository.Source != "github" {
		t.Errorf("Expected repository source 'github', got '%s'", serverDetail.Repository.Source)
	}

	// Verify remotes configuration
	if len(serverDetail.Remotes) != 1 {
		t.Fatalf("Expected 1 remote configuration, got %d", len(serverDetail.Remotes))
	}

	remote := serverDetail.Remotes[0]
	if remote.TransportType != "sse" {
		t.Errorf("Expected transport type 'sse', got '%s'", remote.TransportType)
	}
	if remote.URL != "http://mcp-fs.anonymous.modelcontextprotocol.io/sse" {
		t.Errorf("Expected remote URL 'http://mcp-fs.anonymous.modelcontextprotocol.io/sse', got '%s'", remote.URL)
	}

	// Verify headers
	if len(remote.Headers) != 2 {
		t.Fatalf("Expected 2 headers, got %d", len(remote.Headers))
	}

	// Check secret header (X-API-Key)
	apiKeyHeader := remote.Headers[0]
	if apiKeyHeader.Name != "X-API-Key" {
		t.Errorf("Expected first header name 'X-API-Key', got '%s'", apiKeyHeader.Name)
	}
	if apiKeyHeader.Description != "API key for authentication" {
		t.Errorf("Expected first header description 'API key for authentication', got '%s'", apiKeyHeader.Description)
	}
	if !apiKeyHeader.Required {
		t.Error("Expected X-API-Key header to be required")
	}
	if !apiKeyHeader.Secret {
		t.Error("Expected X-API-Key header to be secret")
	}

	// Check non-secret header (X-Region) with choices
	regionHeader := remote.Headers[1]
	if regionHeader.Name != "X-Region" {
		t.Errorf("Expected second header name 'X-Region', got '%s'", regionHeader.Name)
	}
	if regionHeader.Description != "Service region" {
		t.Errorf("Expected second header description 'Service region', got '%s'", regionHeader.Description)
	}
	if regionHeader.DefaultValue != "us-east-1" {
		t.Errorf("Expected X-Region header default 'us-east-1', got '%v'", regionHeader.DefaultValue)
	}
	expectedChoices := []string{"us-east-1", "eu-west-1", "ap-southeast-1"}
	if len(regionHeader.Choices) != len(expectedChoices) {
		t.Errorf("Expected %d choices, got %d", len(expectedChoices), len(regionHeader.Choices))
	} else {
		for i, expected := range expectedChoices {
			if regionHeader.Choices[i] != expected {
				t.Errorf("Expected choice '%s' at index %d, got '%s'", expected, i, regionHeader.Choices[i])
			}
		}
	}

	// Test conversion to catalog server
	catalogServer := serverDetail.ToCatalogServer()

	// Verify basic conversion
	if catalogServer.Description != "remote example" {
		t.Errorf("Expected catalog server description 'remote example', got '%s'", catalogServer.Description)
	}

	// Verify remote configuration
	if catalogServer.Remote.URL != "http://mcp-fs.anonymous.modelcontextprotocol.io/sse" {
		t.Errorf("Expected catalog remote URL 'http://mcp-fs.anonymous.modelcontextprotocol.io/sse', got '%s'", catalogServer.Remote.URL)
	}
	if catalogServer.Remote.Transport != "sse" {
		t.Errorf("Expected catalog remote transport 'sse', got '%s'", catalogServer.Remote.Transport)
	}

	// Verify headers conversion
	expectedHeaders := map[string]string{
		"X-API-Key": "${X_API_Key}", // Secret header should become template (canonicalized)
		"X-Region":  "us-east-1",    // Non-secret header should use default value
	}
	if len(catalogServer.Remote.Headers) != len(expectedHeaders) {
		t.Errorf("Expected %d headers, got %d", len(expectedHeaders), len(catalogServer.Remote.Headers))
	}
	for key, expectedValue := range expectedHeaders {
		if actualValue, exists := catalogServer.Remote.Headers[key]; !exists {
			t.Errorf("Expected header '%s' to exist", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected header '%s' value '%s', got '%s'", key, expectedValue, actualValue)
		}
	}

	// Verify secrets (only X-API-Key should become a secret)
	if len(catalogServer.Secrets) != 1 {
		t.Errorf("Expected 1 secret, got %d", len(catalogServer.Secrets))
	} else {
		secret := catalogServer.Secrets[0]
		if secret.Name != "io_github_slimslenderslacks/remote.X-API-Key" {
			t.Errorf("Expected secret name 'io_github_slimslenderslacks/remote.X-API-Key', got '%s'", secret.Name)
		}
		if secret.Env != "X_API_Key" {
			t.Errorf("Expected secret env 'X_API_Key', got '%s'", secret.Env)
		}
	}
}
