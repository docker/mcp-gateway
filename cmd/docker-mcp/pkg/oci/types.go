package oci

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/catalog"
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
	RegistryType     string          `json:"registry_type"`     // npm, pypi, oci, nuget, mcpb
	Identifier       string          `json:"identifier"`        // registry name
	Version          string          `json:"version,omitempty"` // tag or digest
	RegistryBaseURL  string          `json:"registry_base_url,omitempty"`
	Env              []KeyValueInput `json:"environment_variables,omitempty"`
	RuntimeOptions   []Argument      `json:"runtime_arguments,omitempty"`
	PackageArguments []Argument      `json:"package_arguments,omitempty"`
}

type InputWithVariables struct {
	Input

	Variables map[string]Input `json:"variables,omitempty"`
}

type Argument struct {
	InputWithVariables

	Type       string `json:"type"` // named, positional
	ValueHint  string `json:"value_hint,omitempty"`
	IsRepeated bool   `json:"is_repeated,omitempty"`
	Name       string `json:"name,omitempty"` // required if named
}

type KeyValueInput struct {
	InputWithVariables

	Name string `json:"name"`
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
	Description  string   `json:"description,omitempty"`
	Value        string   `json:"value,omitempty"`
	Required     bool     `json:"is_required,omitempty"`
	Secret       bool     `json:"is_secret,omitempty"`
	DefaultValue string   `json:"default,omitempty"`
	Choices      []string `json:"choices,omitempty"`
	Format       string   `json:"format,omitempty"` // "string", "number", "boolean", "filepath"
}

// Remote represents a remote server configuration
type RemoteServer struct {
	URL           string          `json:"url,omitempty"`
	TransportType string          `json:"type,omitempty"` // "streamable-http", "sse"
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
			value, secrets, config := getKeyValueInput(envVar, CanonicalizeServerName(sd.Name), false)
			if len(secrets) > 0 {
				// don't need explicit env because secret adds it
				server.Secrets = append(server.Secrets, secrets...)
			} else {
				server.Env = append(server.Env, catalog.Env{
					Name:  envVar.Name,
					Value: value,
				})
				server.Config = mergeConfig(server.Config, CanonicalizeServerName(sd.Name), config)
			}
		}

		// Process package arguments and append positional ones to command
		for _, arg := range pkg.PackageArguments {
			switch arg.Type {
			case "positional":
				value, secrets, configSchema := getInput(arg.InputWithVariables, CanonicalizeServerName(sd.Name))
				server.Command = append(server.Command, value)

				// Add any secrets from the argument
				if len(secrets) > 0 {
					server.Secrets = append(server.Secrets, secrets...)
				}

				// Add any config schema from the argument
				if configSchema != nil {
					server.Config = mergeConfig(server.Config, CanonicalizeServerName(sd.Name), configSchema)
				}
			case "named":
				value, secrets, configSchema := getInput(arg.InputWithVariables, CanonicalizeServerName(sd.Name))
				server.Command = append(server.Command, fmt.Sprintf("--%s", arg.Name), value)

				// Add any secrets from the argument
				if len(secrets) > 0 {
					server.Secrets = append(server.Secrets, secrets...)
				}

				// Add any config schema from the argument
				if configSchema != nil {
					server.Config = mergeConfig(server.Config, CanonicalizeServerName(sd.Name), configSchema)
				}
			}
		}

		// Process runtime arguments
		for _, arg := range pkg.RuntimeOptions {
			// volume arguments have special meaning
			if arg.Type == "named" && (arg.Name == "-v" || arg.Name == "--mount") {
				config, volume := createVolume(arg, CanonicalizeServerName(sd.Name))
				if volume != "" {
					server.Volumes = append(server.Volumes, volume)
				}
				if config != nil {
					server.Config = mergeConfig(server.Config, CanonicalizeServerName(sd.Name), config)
				}
			}
			// TODO support User args explicitly
		}
	}

	// Handle remote configuration if available
	if len(sd.Remotes) > 0 {
		remote := sd.Remotes[0]

		// Convert KeyValueInput headers to string headers
		headers := make(map[string]string)
		headerSecrets := []catalog.Secret{}
		for _, header := range remote.Headers {
			value, secrets, _ := getKeyValueInput(header, CanonicalizeServerName(sd.Name), true)
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

func getKeyValueInput(kvi KeyValueInput, serverName string, useEnvForm bool) (string, []catalog.Secret, map[string]any) {
	if len(kvi.Variables) == 0 {
		if kvi.Secret {
			var templateValue string
			if useEnvForm {
				templateValue = fmt.Sprintf("${%s}", canonicalizeEnvName(kvi.Name))
			} else {
				templateValue = fmt.Sprintf("{{%s}}", kvi.Name)
			}
			secret := catalog.Secret{
				Name: fmt.Sprintf("%s.%s", serverName, kvi.Name),
				Env:  canonicalizeEnvName(kvi.Name),
			}
			return templateValue, []catalog.Secret{secret}, map[string]any{}
		} else if kvi.Value != "" {
			return kvi.Value, []catalog.Secret{}, map[string]any{}
		} else if kvi.DefaultValue != "" {
			// Use default value when no explicit value is provided
			return fmt.Sprintf("%v", kvi.DefaultValue), []catalog.Secret{}, map[string]any{}
		}
		config := map[string]any{
			"type": "object",
			"properties": map[string]any{
				kvi.Name: map[string]any{
					"type": "string",
				},
			},
		}
		return fmt.Sprintf("{{%s}}", fmt.Sprintf("%s.%s", serverName, kvi.Name)), []catalog.Secret{}, config
	}
	value, secrets, config := getInput(kvi.InputWithVariables, serverName)
	return value, secrets, config
}

func getInput(arg InputWithVariables, serverName string) (string, []catalog.Secret, map[string]any) {
	var secrets []catalog.Secret
	var configSchema map[string]any

	// Process the main input
	value := arg.Value
	if arg.Secret {
		// TODO - this is bad practice (visible to docker inspect)
		secret := catalog.Secret{
			Name: fmt.Sprintf("%s.%s", serverName, arg.Description),
			Env:  canonicalizeEnvName(arg.Description),
		}
		secrets = append(secrets, secret)

		// Replace with template reference
		value = fmt.Sprintf("{{%s}}", arg.Description)
	} else if len(arg.Variables) > 0 {
		// This input has variables, create config schema
		properties := make(map[string]any)
		required := []string{}

		for varName, variable := range arg.Variables {
			prop := map[string]any{
				"type":        "string", // Default to string
				"description": variable.Description,
			}

			// Add default value if present
			if variable.DefaultValue != "" {
				prop["default"] = variable.DefaultValue
			}

			// Add format if specified
			if variable.Format != "" {
				prop["format"] = variable.Format
			}

			// Add choices as enum if present
			if len(variable.Choices) > 0 {
				prop["enum"] = variable.Choices
			}

			properties[varName] = prop

			// Add to required if the variable is required
			if variable.Required {
				required = append(required, varName)
			}

			// Handle secret variables
			if variable.Secret {
				secret := catalog.Secret{
					Name: fmt.Sprintf("%s.%s", serverName, varName),
					Env:  canonicalizeEnvName(varName),
				}
				secrets = append(secrets, secret)

				// Replace in value string using secret reference
				value = replaceVariables(value, serverName)
			}
		}

		// Create the config schema
		configSchema = map[string]any{
			"type":       "object",
			"properties": properties,
		}

		if len(required) > 0 {
			configSchema["required"] = required
		}

		// Replace variables in the value string
		value = replaceVariables(value, serverName)
	}

	// If we have a default value and no explicit value, use the default
	if value == "" && arg.DefaultValue != "" {
		value = fmt.Sprintf("%v", arg.DefaultValue)
	}

	return value, secrets, configSchema
}

func canonicalizeEnvName(s string) string {
	return strings.ReplaceAll(s, "-", "_")
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

				if variable.DefaultValue != "" {
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
