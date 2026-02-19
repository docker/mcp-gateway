package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"

	"github.com/docker/mcp-gateway/pkg/registryapi"
)

// ErrIncompatibleServer is returned by TransformToDocker when the server has
// no compatible package type (e.g. no OCI+stdio package and no remote).
var ErrIncompatibleServer = errors.New("incompatible server")

// TransformOption configures the behavior of TransformToDocker.
type TransformOption func(*transformOptions)

type transformOptions struct {
	allowPyPI    bool
	pypiResolver PyPIVersionResolver
}

// WithAllowPyPI controls whether PyPI packages are considered during transformation.
// By default, PyPI packages are allowed.
func WithAllowPyPI(allow bool) TransformOption {
	return func(o *transformOptions) {
		o.allowPyPI = allow
	}
}

// WithPyPIResolver sets the PyPI version resolver used to determine the Python
// version for PyPI packages. If not set, the default Python version is used.
func WithPyPIResolver(resolver PyPIVersionResolver) TransformOption {
	return func(o *transformOptions) {
		o.pypiResolver = resolver
	}
}

// Type aliases for imported types from the registry package
type (
	ServerDetail  = v0.ServerJSON
	RegistryEntry = v0.ServerResponse
)

// Using types from github.com/docker/mcp-gateway/pkg/catalog

// Helper Functions

func extractServerName(fullName string) string {
	// com.docker.mcp/server-name -> com-docker-mcp-server-name
	name := strings.ReplaceAll(fullName, "/", "-")
	name = strings.ReplaceAll(name, ".", "-")
	return name
}

func collectVariables(serverDetail ServerDetail) map[string]model.Input {
	variables := make(map[string]model.Input)

	// Collect from packages
	for _, pkg := range serverDetail.Packages {
		// From package arguments
		for _, arg := range pkg.PackageArguments {
			for k, v := range arg.Variables {
				variables[k] = v
			}
		}
		// From runtime arguments
		for _, arg := range pkg.RuntimeArguments {
			for k, v := range arg.Variables {
				variables[k] = v
			}
		}
		// From environment variables
		for _, envVar := range pkg.EnvironmentVariables {
			// Check if the env var has nested variables (interpolation case)
			for k, v := range envVar.Variables {
				variables[k] = v
			}
			// Also check if the env var itself is a direct secret/config
			// (no value, just a declaration with isSecret/isRequired)
			if envVar.Value == "" && (envVar.IsSecret || envVar.IsRequired || envVar.Description != "") {
				variables[envVar.Name] = envVar.Input
			}
		}
	}

	// Collect from remotes
	for _, remote := range serverDetail.Remotes {
		for _, header := range remote.Headers {
			for k, v := range header.Variables {
				variables[k] = v
			}
		}
	}

	return variables
}

func separateSecretsAndConfig(variables map[string]model.Input) (secrets map[string]model.Input, config map[string]model.Input) {
	secrets = make(map[string]model.Input)
	config = make(map[string]model.Input)

	for k, v := range variables {
		if v.IsSecret {
			secrets[k] = v
		} else {
			config[k] = v
		}
	}

	return secrets, config
}

func buildConfigSchema(configVars map[string]model.Input, serverName string) []any {
	if len(configVars) == 0 {
		return nil
	}

	properties := make(map[string]any)
	var required []string

	for varName, varDef := range configVars {
		jsonType := "string"
		switch varDef.Format {
		case model.FormatNumber:
			jsonType = "number"
		case model.FormatBoolean:
			jsonType = "boolean"
		}

		prop := map[string]any{
			"type":        jsonType,
			"description": varDef.Description,
		}

		// Add optional fields if present
		if varDef.Default != "" {
			prop["default"] = varDef.Default
		}
		if varDef.Placeholder != "" {
			prop["placeholder"] = varDef.Placeholder
		}

		properties[varName] = prop

		if varDef.IsRequired {
			required = append(required, varName)
		}
	}

	result := map[string]any{
		"name":        serverName,
		"type":        "object",
		"description": fmt.Sprintf("Configuration for %s", serverName),
		"properties":  properties,
	}
	if len(required) > 0 {
		result["required"] = required
	}
	return []any{result}
}

func buildSecrets(serverName string, secretVars map[string]model.Input) []Secret {
	var secrets []Secret

	for varName := range secretVars {
		secret := Secret{
			Name: fmt.Sprintf("%s.%s", serverName, varName),
			Env:  strings.ToUpper(varName),
		}

		secrets = append(secrets, secret)
	}

	return secrets
}

func extractImageInfo(pkg model.Package) string {
	if pkg.RegistryType == "oci" && pkg.Transport.Type == "stdio" {
		if pkg.Version != "" {
			return fmt.Sprintf("%s@%s", pkg.Identifier, pkg.Version)
		}
		return pkg.Identifier
	}
	return ""
}

func extractPyPIInfo(pkg model.Package, pythonVersion string) (image string, command []string, volumes []string) {
	if pkg.RegistryType != "pypi" {
		return "", nil, nil
	}

	// Set the uv Docker image based on Python version
	image = pythonVersionToImageTag(pythonVersion)

	// Build uvx command
	command = []string{"uvx"}

	// Add custom registry if specified (and not default PyPI)
	if pkg.RegistryBaseURL != "" && pkg.RegistryBaseURL != "https://pypi.org" {
		command = append(command, "--index-url", pkg.RegistryBaseURL)
	}

	// Add version specifier if present
	if pkg.Version != "" {
		command = append(command, "--from", fmt.Sprintf("%s==%s", pkg.Identifier, pkg.Version))
	}

	// Add the package name
	command = append(command, pkg.Identifier)

	// Add uv cache volume
	volumes = []string{"docker-mcp-uv-cache:/root/.cache/uv"}

	return image, command, volumes
}

func restoreInterpolatedValue(processedValue string, variables map[string]model.Input, serverName string) string {
	result := processedValue

	// Replace {varName} with {{serverName.varName}} for config vars or ${VARNAME} for secrets
	for varName, varDef := range variables {
		placeholder := fmt.Sprintf("{%s}", varName)
		var replacement string
		if varDef.IsSecret {
			replacement = fmt.Sprintf("${%s}", strings.ToUpper(varName))
		} else {
			replacement = fmt.Sprintf("{{%s.%s}}", serverName, varName)
		}
		result = strings.ReplaceAll(result, placeholder, replacement)
	}

	return result
}

func convertEnvVariables(envVars []model.KeyValueInput, configVars map[string]model.Input, serverName string) []Env {
	if len(envVars) == 0 {
		return nil
	}

	var result []Env
	for _, ev := range envVars {
		// Skip direct secret env vars - they should only be in secrets array
		if ev.IsSecret {
			continue
		}

		value := ev.Value
		if len(ev.Variables) > 0 {
			// If there are nested variables, restore interpolation
			value = restoreInterpolatedValue(value, ev.Variables, serverName)
		} else if value == "" {
			// Check if this env var is defined as a config variable
			if _, isConfig := configVars[ev.Name]; isConfig {
				// Use fully qualified interpolation syntax to reference the config variable
				value = fmt.Sprintf("{{%s.%s}}", serverName, ev.Name)
			} else if ev.Default != "" {
				// Otherwise use the default value
				value = ev.Default
			}
		}

		result = append(result, Env{
			Name:  ev.Name,
			Value: value,
		})
	}

	return result
}

func parseRuntimeArg(arg model.Argument, serverName string) string {
	value := arg.Value
	if len(arg.Variables) > 0 {
		value = restoreInterpolatedValue(value, arg.Variables, serverName)
	}

	if arg.Type == model.ArgumentTypeNamed {
		return fmt.Sprintf("%s=%s", arg.Name, value)
	}
	return value
}

func extractUserFromRuntimeArgs(runtimeArgs []model.Argument, serverName string) string {
	for _, arg := range runtimeArgs {
		if arg.Type == model.ArgumentTypeNamed && (arg.Name == "-u" || arg.Name == "--user") {
			value := arg.Value
			if len(arg.Variables) > 0 {
				value = restoreInterpolatedValue(value, arg.Variables, serverName)
			}
			// Extract value after '='
			parts := strings.SplitN(value, "=", 2)
			if len(parts) == 2 {
				return parts[1]
			}
			return value
		}
	}
	return ""
}

func extractVolumesFromRuntimeArgs(runtimeArgs []model.Argument, serverName string) []string {
	var volumes []string

	for _, arg := range runtimeArgs {
		if arg.Type != model.ArgumentTypeNamed {
			continue
		}

		value := arg.Value
		if len(arg.Variables) > 0 {
			value = restoreInterpolatedValue(value, arg.Variables, serverName)
		}

		switch arg.Name {
		case "--mount":
			// For --mount, parse and convert to simple src:dst format
			// Input: "type=bind,src={{source_path}},dst={{target_path}}"
			// Output: "{{source_path}}:{{target_path}}"
			var src, dst string
			parts := strings.Split(value, ",")
			for _, part := range parts {
				kv := strings.SplitN(part, "=", 2)
				if len(kv) == 2 {
					switch kv[0] {
					case "src", "source":
						src = kv[1]
					case "dst", "destination", "target":
						dst = kv[1]
					}
				}
			}
			if src != "" && dst != "" {
				volumes = append(volumes, fmt.Sprintf("%s:%s", src, dst))
			} else {
				// Fallback to full value if parsing fails
				volumes = append(volumes, value)
			}
		case "-v":
			// For -v, extract value after '=' if present
			parts := strings.SplitN(value, "=", 2)
			if len(parts) == 2 {
				volumes = append(volumes, parts[1])
			} else {
				volumes = append(volumes, value)
			}
		}
	}

	return volumes
}

func convertPackageArgsToCommand(packageArgs []model.Argument, serverName string) []string {
	if len(packageArgs) == 0 {
		return nil
	}

	var command []string
	for _, arg := range packageArgs {
		command = append(command, parseRuntimeArg(arg, serverName))
	}

	return command
}

func convertRemote(remote model.Transport, serverName string) Remote {
	catalogRemote := Remote{
		URL:       remote.URL,
		Transport: remote.Type,
	}

	if len(remote.Headers) > 0 {
		headers := make(map[string]string)
		for _, header := range remote.Headers {
			value := header.Value
			if len(header.Variables) > 0 {
				value = restoreInterpolatedValue(value, header.Variables, serverName)
			}
			headers[header.Name] = value
		}
		catalogRemote.Headers = headers
	}

	return catalogRemote
}

func getPublisherProvidedMeta(meta *v0.ServerMeta) map[string]any {
	if meta == nil {
		return nil
	}
	return meta.PublisherProvided
}

// TransformToDocker transforms a ServerDetail (community format) to Server (catalog format)
func TransformToDocker(ctx context.Context, serverDetail ServerDetail, opts ...TransformOption) (*Server, error) {
	options := transformOptions{
		allowPyPI: true,
	}
	for _, opt := range opts {
		opt(&options)
	}

	serverName := extractServerName(serverDetail.Name)

	// Find first OCI or PyPI package with stdio transport, preferring OCI
	var pkg *model.Package
	for i := range serverDetail.Packages {
		if serverDetail.Packages[i].RegistryType == "oci" &&
			serverDetail.Packages[i].Transport.Type == "stdio" {
			pkg = &serverDetail.Packages[i]
			break
		}
	}
	if pkg == nil && options.allowPyPI {
		for i := range serverDetail.Packages {
			if serverDetail.Packages[i].RegistryType == "pypi" &&
				serverDetail.Packages[i].Transport.Type == "stdio" {
				pkg = &serverDetail.Packages[i]
				break
			}
		}
	}

	var remote *model.Transport
	if len(serverDetail.Remotes) > 0 {
		remote = &serverDetail.Remotes[0]
	}

	variables := collectVariables(serverDetail)
	secretVars, configVars := separateSecretsAndConfig(variables)

	server := &Server{
		Name:        serverName,
		Title:       serverDetail.Title,
		Description: serverDetail.Description,
	}

	// Add image and command for OCI or PyPI package
	if pkg != nil {
		switch pkg.RegistryType {
		case "oci":
			if image := extractImageInfo(*pkg); image != "" {
				server.Image = image
				server.Type = "server"
			}
		case "pypi":
			var pythonVersion string
			if options.pypiResolver != nil {
				pythonVersion = options.pypiResolver(ctx, pkg.Identifier, pkg.Version, pkg.RegistryBaseURL)
			}
			image, command, volumes := extractPyPIInfo(*pkg, pythonVersion)
			if image != "" {
				server.Image = image
				server.Command = command
				server.Volumes = volumes
				server.Type = "server"
			}
		default:
			return nil, fmt.Errorf("unsupported registry type: %s", pkg.RegistryType)
		}
	}

	// Add remote if present
	if remote != nil {
		remoteVal := convertRemote(*remote, serverName)
		server.Remote = remoteVal
		server.Type = "remote"
	}

	// Validate that we have at least one way to run the server
	if server.Image == "" && server.Remote.URL == "" {
		return nil, fmt.Errorf("%w: no compatible packages for %s", ErrIncompatibleServer, serverDetail.Name)
	}

	// Add config schema if we have config variables
	if len(configVars) > 0 {
		server.Config = buildConfigSchema(configVars, serverName)
	}

	// Add secrets if we have secret variables
	if len(secretVars) > 0 {
		server.Secrets = buildSecrets(serverName, secretVars)
	}

	// Add environment variables
	if pkg != nil && len(pkg.EnvironmentVariables) > 0 {
		server.Env = convertEnvVariables(pkg.EnvironmentVariables, configVars, serverName)
	}

	// Add package arguments
	if pkg != nil && len(pkg.PackageArguments) > 0 {
		if pkg.RegistryType == "pypi" {
			// For PyPI: append package args to the uvx command
			server.Command = append(server.Command, convertPackageArgsToCommand(pkg.PackageArguments, serverName)...)
		} else {
			// For OCI: package args become the full command
			server.Command = convertPackageArgsToCommand(pkg.PackageArguments, serverName)
		}
	}

	// Add user from runtime arguments
	if pkg != nil {
		if user := extractUserFromRuntimeArgs(pkg.RuntimeArguments, serverName); user != "" {
			server.User = user
		}
	}

	// Add volumes from runtime arguments
	if pkg != nil {
		if volumes := extractVolumesFromRuntimeArgs(pkg.RuntimeArguments, serverName); len(volumes) > 0 {
			server.Volumes = append(server.Volumes, volumes...)
		}
	}

	// Add metadata from publisher-provided
	if publisherMeta := getPublisherProvidedMeta(serverDetail.Meta); publisherMeta != nil {
		if oauthData, ok := publisherMeta["oauth"]; ok {
			// Try to convert to OAuth
			if oauthJSON, err := json.Marshal(oauthData); err == nil {
				var oauth OAuth
				if err := json.Unmarshal(oauthJSON, &oauth); err == nil {
					server.OAuth = &oauth
				}
			}
		}
	}

	// Add icon
	if len(serverDetail.Icons) > 0 {
		server.Icon = serverDetail.Icons[0].Src
	}

	// Add registry URL metadata
	if serverDetail.Name != "" && serverDetail.Version != "" {
		server.Metadata = &Metadata{
			RegistryURL: registryapi.BuildServerURL(serverDetail.Name, serverDetail.Version),
		}
	}

	return server, nil
}
