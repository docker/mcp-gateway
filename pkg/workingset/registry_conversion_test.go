package workingset

import (
	"testing"

	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

func TestConvertRegistryServerToCatalog_BasicOCI(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/test-server",
			Description: "A test MCP server",
			Title:       "Test Server",
			Version:     "1.0.0",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/test-server:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	assert.Equal(t, "io-github-user-test-server", catalogServer.Name)
	assert.Equal(t, "server", catalogServer.Type)
	assert.Equal(t, "ghcr.io/user/test-server:1.0.0", catalogServer.Image)
	assert.Equal(t, "A test MCP server", catalogServer.Description)
	assert.Equal(t, "Test Server", catalogServer.Title)

	// Verify registry URL is set in metadata
	require.NotNil(t, catalogServer.Metadata)
	assert.Equal(t, "https://registry.modelcontextprotocol.io/v0/servers/io.github.user%2Ftest-server/versions/1.0.0", catalogServer.Metadata.RegistryURL)
}

func TestConvertRegistryServerToCatalog_WithIcon(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/test",
			Description: "Test server",
			Icons: []model.Icon{
				{
					Src: "https://example.com/icon.png",
				},
			},
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/test:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	assert.Equal(t, "https://example.com/icon.png", catalogServer.Icon)
}

func TestConvertRegistryServerToCatalog_WithVolumesAndUser(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/test",
			Description: "Test server",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/test:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
					RuntimeArguments: []model.Argument{
						{
							Type: model.ArgumentTypeNamed,
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Value: "/host/path:/container/path",
								},
							},
							Name: "-v",
						},
						{
							Type: model.ArgumentTypeNamed,
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Value: "1000:1000",
								},
							},
							Name: "--user",
						},
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	assert.Len(t, catalogServer.Volumes, 1)
	assert.Equal(t, "/host/path:/container/path", catalogServer.Volumes[0])
	assert.Equal(t, "1000:1000", catalogServer.User)
}

func TestConvertRegistryServerToCatalog_WithVolumeVariables(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.arm/arm-mcp",
			Description: "Arm MCP server",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "docker.io/armlimited/arm-mcp:1.0.1",
					Transport: model.Transport{
						Type: "stdio",
					},
					RuntimeArguments: []model.Argument{
						{
							Type: model.ArgumentTypeNamed,
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Description: "Mount a local directory into the container",
									Value:       "{workspace_path}:/workspace",
									IsRequired:  true,
								},
								Variables: map[string]model.Input{
									"workspace_path": {
										Description: "Local directory to make accessible",
										IsRequired:  true,
										Format:      "filepath",
										Placeholder: "/path/to/your/project",
									},
								},
							},
							Name: "-v",
						},
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	// Volume should be extracted with placeholder converted to {{serverName.var}} format
	assert.Len(t, catalogServer.Volumes, 1)
	assert.Equal(t, "{{io-github-arm-arm-mcp.workspace_path}}:/workspace", catalogServer.Volumes[0])

	// Config should contain workspace_path variable with server name as config name
	assert.Len(t, catalogServer.Config, 1)
	configItem := catalogServer.Config[0].(map[string]any)
	assert.Equal(t, "io-github-arm-arm-mcp", configItem["name"]) // Uses full normalized server name
	assert.Equal(t, "Configuration for io-github-arm-arm-mcp", configItem["description"])

	properties := configItem["properties"].(map[string]any)
	workspaceProp := properties["workspace_path"].(map[string]any)
	assert.Equal(t, "string", workspaceProp["type"])
	assert.Equal(t, "Local directory to make accessible", workspaceProp["description"])
	assert.Equal(t, "/path/to/your/project", workspaceProp["placeholder"])

	required := configItem["required"].([]string)
	assert.Contains(t, required, "workspace_path")
}

func TestConvertRegistryServerToCatalog_WithCommand(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/test",
			Description: "Test server",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/test:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
					PackageArguments: []model.Argument{
						{
							Type: model.ArgumentTypePositional,
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Value: "--verbose",
								},
							},
						},
						{
							Type: model.ArgumentTypePositional,
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Value: "--config=/etc/config",
								},
							},
						},
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	assert.Len(t, catalogServer.Command, 2)
	assert.Equal(t, "--verbose", catalogServer.Command[0])
	assert.Equal(t, "--config=/etc/config", catalogServer.Command[1])
}

func TestConvertRegistryServerToCatalog_WithSecrets(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/test",
			Description: "Test server",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/test:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
					EnvironmentVariables: []model.KeyValueInput{
						{
							Name: "API_KEY",
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Description: "API key for authentication",
									IsSecret:    true,
									IsRequired:  true,
								},
							},
						},
						{
							Name: "DATABASE_PASSWORD",
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Description: "Database password",
									IsSecret:    true,
									IsRequired:  true,
								},
							},
						},
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	assert.Len(t, catalogServer.Secrets, 2)
	// Secrets are in map iteration order, so we need to check both possibilities
	secretNames := []string{catalogServer.Secrets[0].Name, catalogServer.Secrets[1].Name}
	assert.Contains(t, secretNames, "io-github-user-test.API_KEY")
	assert.Contains(t, secretNames, "io-github-user-test.DATABASE_PASSWORD")
	// Check env vars match
	for _, secret := range catalogServer.Secrets {
		switch secret.Name {
		case "io-github-user-test.API_KEY":
			assert.Equal(t, "API_KEY", secret.Env)
		case "io-github-user-test.DATABASE_PASSWORD":
			assert.Equal(t, "DATABASE_PASSWORD", secret.Env)
		}
	}
}

func TestConvertRegistryServerToCatalog_WithEnvironmentVariables(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/test",
			Description: "Test server",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/test:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
					EnvironmentVariables: []model.KeyValueInput{
						{
							Name: "LOG_LEVEL",
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Description: "Logging level",
									Value:       "info",
									IsRequired:  true,
								},
							},
						},
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	assert.Len(t, catalogServer.Env, 1)
	assert.Equal(t, "LOG_LEVEL", catalogServer.Env[0].Name)
	assert.Equal(t, "info", catalogServer.Env[0].Value)
}

func TestConvertRegistryServerToCatalog_WithConfigVariables(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/test",
			Description: "Test server",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/test:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
					EnvironmentVariables: []model.KeyValueInput{
						{
							Name: "DATABASE_URL",
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Description: "Database connection string",
									IsRequired:  true,
									Format:      model.FormatString,
									Value:       "{host}:{port}/{db}",
								},
								Variables: map[string]model.Input{
									"host": {
										Description: "Database host",
										Default:     "localhost",
										Format:      model.FormatString,
									},
									"port": {
										Description: "Database port",
										Default:     "5432",
										Format:      model.FormatNumber,
									},
									"db": {
										Description: "Database name",
										IsRequired:  true,
										Format:      model.FormatString,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	// Verify Env entry is created with converted placeholders
	// This is critical - gateway exports env vars from Spec.Env
	assert.Len(t, catalogServer.Env, 1)
	assert.Equal(t, "DATABASE_URL", catalogServer.Env[0].Name)
	assert.Equal(t, "{{io-github-user-test.host}}:{{io-github-user-test.port}}/{{io-github-user-test.db}}", catalogServer.Env[0].Value)

	assert.Len(t, catalogServer.Config, 1)
	configItem := catalogServer.Config[0].(map[string]any)

	// Config name should be full normalized server name
	assert.Equal(t, "io-github-user-test", configItem["name"])
	assert.Equal(t, "Configuration for io-github-user-test", configItem["description"])
	assert.Equal(t, "object", configItem["type"])

	properties := configItem["properties"].(map[string]any)
	assert.Len(t, properties, 3)

	// Check host property
	hostProp := properties["host"].(map[string]any)
	assert.Equal(t, "string", hostProp["type"])
	assert.Equal(t, "localhost", hostProp["default"])

	// Check port property
	portProp := properties["port"].(map[string]any)
	assert.Equal(t, "number", portProp["type"])
	assert.Equal(t, "5432", portProp["default"])

	// Check db property (required)
	dbProp := properties["db"].(map[string]any)
	assert.Equal(t, "string", dbProp["type"])

	required := configItem["required"].([]string)
	assert.Contains(t, required, "db")
}

func TestConvertRegistryServerToCatalog_MultipleSimpleEnvVars(t *testing.T) {
	// This test verifies that multiple simple environment variables are merged
	// into a single config item named after the server (servername.field format)
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/obsidian",
			Description: "Obsidian server",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/obsidian:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
					EnvironmentVariables: []model.KeyValueInput{
						{
							Name: "API_URLS",
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Description: "API URLs",
									IsRequired:  true,
								},
							},
						},
						{
							Name: "MCP_TRANSPORTS",
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Description: "Transports",
									Default:     "stdio,http",
								},
							},
						},
						{
							Name: "MCP_HTTP_PORT",
							InputWithVariables: model.InputWithVariables{
								Input: model.Input{
									Description: "Port",
									Default:     "3000",
									Format:      model.FormatNumber,
								},
							},
						},
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	// Should have 1 config item named after server with all properties merged
	assert.Len(t, catalogServer.Config, 1)
	configItem := catalogServer.Config[0].(map[string]any)

	// Config name is full normalized server name
	assert.Equal(t, "io-github-user-obsidian", configItem["name"])
	assert.Equal(t, "Configuration for io-github-user-obsidian", configItem["description"])
	assert.Equal(t, "object", configItem["type"])

	// Properties contain all 3 env vars (using original case)
	properties := configItem["properties"].(map[string]any)
	assert.Len(t, properties, 3)

	// Check API_URLS property
	apiUrlsProp := properties["API_URLS"].(map[string]any)
	assert.Equal(t, "string", apiUrlsProp["type"])
	assert.Equal(t, "API URLs", apiUrlsProp["description"])

	// Check MCP_TRANSPORTS property
	transportsProp := properties["MCP_TRANSPORTS"].(map[string]any)
	assert.Equal(t, "string", transportsProp["type"])
	assert.Equal(t, "stdio,http", transportsProp["default"])

	// Check MCP_HTTP_PORT property
	portProp := properties["MCP_HTTP_PORT"].(map[string]any)
	assert.Equal(t, "number", portProp["type"])
	assert.Equal(t, "3000", portProp["default"])

	// API_URLS is required
	required := configItem["required"].([]string)
	assert.Contains(t, required, "API_URLS")
}

func TestConvertRegistryServerToCatalog_NoOCIPackages(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/test",
			Description: "Test server",
			Packages: []model.Package{
				{
					RegistryType: "npm",
					Identifier:   "@user/test-server",
					Version:      "1.0.0",
				},
			},
		},
	}

	_, err := ConvertRegistryServerToCatalog(serverResp)
	require.Error(t, err)
	assert.ErrorIs(t, err, catalog.ErrIncompatibleServer)
}

func TestConvertRegistryServerToCatalog_MultipleOCIPackages(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/test",
			Description: "Test server",
			Packages: []model.Package{
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/test:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
				},
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/test-alt:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	// Should return the first OCI package
	assert.Equal(t, "ghcr.io/user/test:1.0.0", catalogServer.Image)
}

func TestConvertRegistryServerToCatalog_MixedPackageTypes(t *testing.T) {
	serverResp := &v0.ServerResponse{
		Server: v0.ServerJSON{
			Name:        "io.github.user/test",
			Description: "Test server",
			Packages: []model.Package{
				{
					RegistryType: "npm",
					Identifier:   "@user/test-server",
					Version:      "1.0.0",
				},
				{
					RegistryType: "oci",
					Identifier:   "ghcr.io/user/test:1.0.0",
					Transport: model.Transport{
						Type: "stdio",
					},
				},
			},
		},
	}

	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	require.NoError(t, err)

	// Should return the OCI package, ignoring npm
	assert.Equal(t, "ghcr.io/user/test:1.0.0", catalogServer.Image)
}

// TestNormalizeServerName and TestInferJSONType removed
// These tested internal helper functions that are now in pkg/catalog
