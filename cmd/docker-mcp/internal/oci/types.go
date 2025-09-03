package oci

import (
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

// ServerDetail represents the complete server definition based on the MCP registry schema
type ServerDetail struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	VersionDetail *VersionDetail `json:"version_detail,omitempty"`
	Status        string         `json:"status,omitempty"` // "active", "deprecated", or "deleted"
	Repository    *Repository    `json:"repository,omitempty"`
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
	RegistryType     string            `json:"registry_type"`
	Identifier       string            `json:"identifier"`
	Version          string            `json:"version,omitempty"`
	Args             []string          `json:"args,omitempty"`
	Env              map[string]any    `json:"env,omitempty"`
	RuntimeOptions   *RuntimeOptions   `json:"runtime_options,omitempty"`
	TransportOptions *TransportOptions `json:"transport_options,omitempty"`
	Inputs           []Input           `json:"inputs,omitempty"`
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
	Headers map[string]string `json:"headers,omitempty"`
}

// Input represents input configuration
type Input struct {
	Name         string        `json:"name"`
	Type         string        `json:"type"` // "positional", "named", "secret", "configurable"
	Description  string        `json:"description,omitempty"`
	Required     bool          `json:"required,omitempty"`
	DefaultValue any           `json:"default_value,omitempty"`
	Options      []InputOption `json:"options,omitempty"`
}

// InputOption represents an option for input validation
type InputOption struct {
	Value       any    `json:"value"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// Remote represents a remote server configuration
type RemoteServer struct {
	URL           string            `json:"url"`
	TransportType string            `json:"transport_type,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Inputs        []Input           `json:"inputs,omitempty"`
}

// Remote alias for consistency with existing code
type Remote = RemoteServer

// ToCatalogServer converts an OCI ServerDetail to a catalog.Server
func (sd *ServerDetail) ToCatalogServer() catalog.Server {
	server := catalog.Server{}

	// Extract image from the first package if available
	if len(sd.Packages) > 0 {
		pkg := sd.Packages[0]
		server.Image = fmt.Sprintf("%s:%s", pkg.Identifier, pkg.Version)

		// Set command and environment from runtime options
		if pkg.RuntimeOptions != nil {
			server.Command = pkg.RuntimeOptions.Command

			// Convert env map to env slice
			if pkg.RuntimeOptions.Env != nil {
				for key, value := range pkg.RuntimeOptions.Env {
					if strVal, ok := value.(string); ok {
						server.Env = append(server.Env, catalog.Env{
							Name:  key,
							Value: strVal,
						})
					}
				}
			}
		}

		// Set transport options
		if pkg.TransportOptions != nil {
			server.Remote = catalog.Remote{
				Transport: pkg.TransportOptions.Type,
				Headers:   pkg.TransportOptions.Headers,
			}
		}

		// Convert inputs to secrets for credential-type inputs
		for _, input := range pkg.Inputs {
			if input.Type == "secret" {
				server.Secrets = append(server.Secrets, catalog.Secret{
					Name: input.Name,
					Env:  strings.ToUpper(input.Name),
				})
			}
		}
	}

	// Handle remote configuration if available
	if len(sd.Remotes) > 0 {
		remote := sd.Remotes[0]
		server.Remote = catalog.Remote{
			URL:       remote.URL,
			Transport: remote.TransportType,
			Headers:   remote.Headers,
		}

		// Convert remote inputs to secrets
		for _, input := range remote.Inputs {
			if input.Type == "secret" {
				server.Secrets = append(server.Secrets, catalog.Secret{
					Name: input.Name,
					Env:  strings.ToUpper(input.Name),
				})
			}
		}
	}

	return server
}
