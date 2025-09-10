package oci

import (
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

// ServerDetail represents the complete server definition based on the MCP registry schema
type ServerDetail struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Version       string         `json:"version"`
	VersionDetail *VersionDetail `json:"version_detail,omitempty"`
	Status        string         `json:"status,omitempty"` // "active", "deprecated", or "deleted"
	Repository    Repository     `json:"repository,omitempty"`
	Packages      []Package      `json:"packages,omitempty"`
	Remotes       []Remote       `json:"remotes,omitempty"`
	Meta          map[string]any `json:"_meta,omitempty"`
}

// Repository contains repository information for the server
type Repository struct {
	URL    string `json:"url,omitempty"`
	Source string `json:"source,omitempty"`
	ID     string `json:"id,omitempty"`
}

// VersionDetail contains version information
type VersionDetail struct {
	Version string `json:"version"`
}

// Package represents a package definition
type Package struct {
	RegistryType     string          `json:"registry_type"`
	Identifier       string          `json:"identifier"`
	Version          string          `json:"version,omitempty"`
	Args             []string        `json:"args,omitempty"`
	Env              []KeyValueInput `json:"environment_variables,omitempty"`
	RuntimeOptions   []Argument      `json:"runtime_arguments,omitempty"`
	PackageArguments []Argument      `json:"package_arguments,omitempty"`
	Inputs           []Input         `json:"inputs,omitempty"`
}

type Argument struct {
	Input
	Variables map[string]Input `json:"variables,omitempty"`

	Type       string `json:"type"`
	ValueHint  string `json:"value_hint,omitempty"`
	IsRepeated bool   `json:"is_repeated,omitempty"`
	Name       string `json:"name,omitempty"`
}

type KeyValueInput struct {
	Input
	Name      string           `json:"name"`
	Variables map[string]Input `json:"variables,omitempty"`
}

// RuntimeOptions contains runtime configuration
type RuntimeOptions struct {
	Command []string       `json:"command,omitempty"`
	Args    []string       `json:"args,omitempty"`
	Env     map[string]any `json:"env,omitempty"`
	WorkDir string         `json:"work_dir,omitempty"`
}

// TransportOptions contains transport configuration
type TransportOptions struct {
	Type    string            `json:"type,omitempty"` // "stdio", "sse", etc.
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// Input represents input configuration
type Input struct {
	Type         string   `json:"type,omitempty"` // "positional", "named", "secret", "configurable"
	Description  string   `json:"description,omitempty"`
	Value        string   `json:"value,omitempty"`
	Required     bool     `json:"is_required,omitempty"`
	Secret       bool     `json:"is_secret,omitempty"`
	DefaultValue any      `json:"default,omitempty"`
	Choices      []string `json:"choices,omitempty"`
	Format       string   `json:"format,omitempty"`
}

// InputOption represents an option for input validation
type InputOption struct {
	Value       any    `json:"value,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// Remote represents a remote server configuration
type RemoteServer struct {
	URL           string                   `json:"url,omitempty"`
	TransportType string                   `json:"transport_type,omitempty"`
	Headers       map[string]KeyValueInput `json:"headers,omitempty"`
}

// Remote alias for consistency with existing code
type Remote = RemoteServer

// ToCatalogServer converts an OCI ServerDetail to a catalog.Server
func (sd *ServerDetail) ToCatalogServer() catalog.Server {
	server := catalog.Server{
		Description: sd.Description,
		Name:        sd.Name,
	}

	// Extract image from the first package if available
	if len(sd.Packages) > 0 {
		pkg := sd.Packages[0]
		server.Image = fmt.Sprintf("%s:%s", pkg.Identifier, pkg.Version)

		// Convert environment variables to secrets, env vars, and config schemas
		for _, envVar := range pkg.Env {
			if envVar.Secret || envVar.Type == "secret" {
				server.Secrets = append(server.Secrets, catalog.Secret{
					Name: envVar.Name,
					Env:  envVar.Name,
				})
			} else if envVar.Type == "configurable" {
				// Convert configurable input to JSON schema object
				schema := map[string]any{
					"type": "object",
					"properties": map[string]any{
						envVar.Name: map[string]any{
							"type":        "string", // Default to string, could be enhanced based on input validation
							"description": envVar.Description,
						},
					},
					"required": []string{},
				}
				if envVar.Required {
					schema["required"] = []string{envVar.Name}
				}
				server.Config = append(server.Config, schema)
			} else {
				// Add as environment variable
				value := envVar.Value
				if value == "" {
					value = fmt.Sprintf("%v", envVar.DefaultValue)
				}
				server.Env = append(server.Env, catalog.Env{
					Name:  envVar.Name,
					Value: value,
				})
			}
		}

		// Process package arguments and append positional ones to command
		for _, arg := range pkg.PackageArguments {
			if arg.Type == "positional" {
				server.Command = append(server.Command, arg.Value)
			}
		}

		// Process runtime arguments for volume mounting
		for _, arg := range pkg.RuntimeOptions {
			if arg.Type == "named" && (arg.Name == "-v" || arg.Name == "--mount") {
				config, volume := createVolume(arg)
				if volume != "" {
					server.Volumes = append(server.Volumes, volume)
				}
				if config != nil {
					server.Config = append(server.Config, config)
				}
			}
		}
	}

	// Handle remote configuration if available
	if len(sd.Remotes) > 0 {
		remote := sd.Remotes[0]
		server.Remote = catalog.Remote{
			URL:       remote.URL,
			Transport: remote.TransportType,
		}
	}

	return server
}

// createVolume creates both a config schema and volume string from a runtime argument
func createVolume(arg Argument) (map[string]any, string) {
	var config map[string]any
	var volume string

	if arg.Name == "--mount" || arg.Name == "-v" {
		volume = arg.Value

		// Create config schema from the argument's variables
		if len(arg.Variables) > 0 {
			properties := make(map[string]any)
			required := []string{}

			for varName, variable := range arg.Variables {
				prop := map[string]any{
					"type":        "string", // Default to string
					"description": variable.Description,
				}

				if variable.DefaultValue != nil {
					prop["default"] = variable.DefaultValue
				}

				if variable.Format != "" {
					prop["format"] = variable.Format
				}

				properties[varName] = prop

				if variable.Required {
					required = append(required, varName)
				}
			}

			config = map[string]any{
				"type":       "object",
				"properties": properties,
				"required":   required,
			}
		}
	}

	return config, volume
}
