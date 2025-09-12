package oci

import (
	"fmt"
	"regexp"
	"strings"

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
	URL           string          `json:"url,omitempty"`
	TransportType string          `json:"type,omitempty"`
	Headers       []KeyValueInput `json:"headers,omitempty"`
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
				config, volume := createVolume(arg, CanonicalizeServerName(sd.Name))
				if volume != "" {
					server.Volumes = append(server.Volumes, volume)
				}
				if config != nil {
					server.Config = mergeConfig(server.Config, CanonicalizeServerName(sd.Name), config)
				}
			}
		}
	}

	// Handle remote configuration if available
	if len(sd.Remotes) > 0 {
		remote := sd.Remotes[0]

		// Convert KeyValueInput headers to string headers
		headers := make(map[string]string)
		headerSecrets := []catalog.Secret{}
		for _, header := range remote.Headers {
			value, secrets := getHeaderInput(header)
			headers[header.Name] = value
			headerSecrets = append(headerSecrets, secrets...)
		}

		server.Remote = catalog.Remote{
			URL:       remote.URL,
			Transport: remote.TransportType,
			Headers:   headers,
		}
		if len(headerSecrets) > 0 {
			server.Secrets = headerSecrets
		}
	}

	return server
}

func getHeaderInput(header KeyValueInput) (string, []catalog.Secret) {
	// If this is a secret with no variables, return template expression and secret
	if header.Secret && len(header.Variables) == 0 {
		templateValue := fmt.Sprintf("${%s}", header.Name)
		secrets := catalog.Secret{
			Name: header.Name,
			Env:  header.Name,
		}
		return templateValue, []catalog.Secret{secrets}
	} else if header.Value != "" {
		return header.Value, []catalog.Secret{}
	} else if header.Description != "" {
		return header.DefaultValue.(string), []catalog.Secret{}
	}

	// For non-secret headers, return the value directly
	return header.Value, []catalog.Secret{}
}

// createVolume creates both a config schema and volume string from a runtime argument
func createVolume(arg Argument, serverName string) (map[string]any, string) {
	var config map[string]any
	var volume string

	if arg.Name == "--mount" || arg.Name == "-v" {
		volume = replaceVariables(parseVolumeMount(arg.Value), serverName)

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

// mergeConfig merges a new config into the existing config slice, creating a top-level
// schema object with name, type "object", and properties fields
func mergeConfig(existingConfig []any, serverName string, newConfig map[string]any) []any {
	// If there's no existing config, create a new schema object
	if len(existingConfig) == 0 {
		schemaObject := map[string]any{
			"name":       serverName,
			"type":       "object",
			"properties": map[string]any{},
		}
		// Merge new properties if they exist
		if newProperties, hasProps := newConfig["properties"].(map[string]any); hasProps {
			schemaObject["properties"] = newProperties
		}
		return []any{schemaObject}
	}

	// Find existing server config object by name
	for _, configItem := range existingConfig {
		if configMap, ok := configItem.(map[string]any); ok {
			if name, hasName := configMap["name"].(string); hasName && name == serverName {
				// Merge properties into the existing server config
				if properties, hasProps := configMap["properties"].(map[string]any); hasProps {
					if newProperties, newHasProps := newConfig["properties"].(map[string]any); newHasProps {
						// Merge new properties into existing properties
						for key, value := range newProperties {
							properties[key] = value
						}
					}
				} else {
					// Initialize properties if they don't exist
					if newProperties, newHasProps := newConfig["properties"].(map[string]any); newHasProps {
						configMap["properties"] = newProperties
					}
				}
				return existingConfig
			}
		}
	}

	// If no existing config for this server, append a new schema object
	schemaObject := map[string]any{
		"name":       serverName,
		"type":       "object",
		"properties": map[string]any{},
	}
	if newProperties, hasProps := newConfig["properties"].(map[string]any); hasProps {
		schemaObject["properties"] = newProperties
	}
	return append(existingConfig, schemaObject)
}

// parseVolumeMount parses a volume mount string and converts it to src:dst format if it's a bind mount
func parseVolumeMount(value string) string {
	// If the string doesn't contain type=bind, return as-is
	if !strings.Contains(value, "type=bind") {
		return value
	}

	// Parse comma-separated values
	parts := strings.Split(value, ",")
	mountOptions := make(map[string]string)

	for _, part := range parts {
		// Parse each part as "X=Y"
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			mountOptions[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	// Check for required src and dst entries
	src, hasSrc := mountOptions["src"]
	dst, hasDst := mountOptions["dst"]

	if !hasSrc || !hasDst {
		// Return original value if both src and dst are not present
		return value
	}

	// Return in src:dst format
	return fmt.Sprintf("%s:%s", src, dst)
}

// replaceVariables replaces {X} patterns with {{serverName.X}} in the input string
func replaceVariables(input, serverName string) string {
	// Use regex to find all {X} patterns and replace with {{serverName.X}}
	re := regexp.MustCompile(`\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract the variable name (remove the braces)
		varName := match[1 : len(match)-1]
		return fmt.Sprintf("{{%s.%s}}", serverName, varName)
	})
}

// canonicalizeServerName replaces all dots in a string with underscores
func CanonicalizeServerName(serverName string) string {
	return strings.ReplaceAll(serverName, ".", "_")
}
