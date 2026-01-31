package workingset

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/policy"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/pkg/sliceutil"
	"github.com/docker/mcp-gateway/pkg/validate"
)

const CurrentWorkingSetVersion = 1

// WorkingSet represents a collection of MCP servers and their configurations
type WorkingSet struct {
	Version int               `yaml:"version" json:"version" validate:"required,min=1,max=1"`
	ID      string            `yaml:"id" json:"id" validate:"required"`
	Name    string            `yaml:"name" json:"name" validate:"required,min=1"`
	Servers []Server          `yaml:"servers" json:"servers" validate:"dive"`
	Secrets map[string]Secret `yaml:"secrets,omitempty" json:"secrets,omitempty" validate:"dive"`
	// Policy describes the policy decision for this working set.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`
}

type ServerType string

const (
	ServerTypeRegistry ServerType = "registry"
	ServerTypeImage    ServerType = "image"
	ServerTypeRemote   ServerType = "remote"
)

// Server represents a server configuration in a working set
type Server struct {
	Type    ServerType     `yaml:"type" json:"type" validate:"required,oneof=registry image remote"`
	Config  map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
	Secrets string         `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Tools   ToolList       `yaml:"tools,omitempty" json:"tools"` // See IsZero() below
	// Policy describes the policy decision for this server.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`

	// ServerTypeRegistry only
	Source string `yaml:"source,omitempty" json:"source,omitempty" validate:"required_if=Type registry"`

	// ServerTypeImage only
	Image string `yaml:"image,omitempty" json:"image,omitempty" validate:"required_if=Type image"`

	// ServerTypeRemote only
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty" validate:"required_if=Type remote"`

	// Optional snapshot of the server schema
	Snapshot *ServerSnapshot `yaml:"snapshot,omitempty" json:"snapshot,omitempty"`
}

type SecretProvider string

const (
	SecretProviderDockerDesktop SecretProvider = "docker-desktop-store"
)

// Secret represents a secret configuration in a working set
type Secret struct {
	Provider SecretProvider `yaml:"provider" json:"provider" validate:"required,oneof=docker-desktop-store"`
}

type ServerSnapshot struct {
	Server catalog.Server `yaml:"server" json:"server"`
}

type ToolList []string

// Needed for proper YAML encoding with omitempty. YAML defaults IsZero to true when a slice is empty, but we only want it on nil.
// This IsZero() + omitempty matches json behavior without omitempty.
func (tools ToolList) IsZero() bool {
	return tools == nil
}

func NewFromDb(dbSet *db.WorkingSet) WorkingSet {
	servers := make([]Server, len(dbSet.Servers))
	for i, server := range dbSet.Servers {
		servers[i] = Server{
			Type:    ServerType(server.Type),
			Config:  server.Config,
			Secrets: server.Secrets,
			Tools:   server.Tools,
		}
		if server.Type == "registry" {
			servers[i].Source = server.Source
		}
		if server.Type == "image" {
			servers[i].Image = server.Image
		}
		if server.Type == "remote" {
			servers[i].Endpoint = server.Endpoint
		}

		if server.Snapshot != nil {
			servers[i].Snapshot = &ServerSnapshot{
				Server: server.Snapshot.Server,
			}
		}
	}

	secrets := make(map[string]Secret)
	for name, secret := range dbSet.Secrets {
		secrets[name] = Secret{
			Provider: SecretProvider(secret.Provider),
		}
	}

	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      dbSet.ID,
		Name:    dbSet.Name,
		Servers: servers,
		Secrets: secrets,
	}

	return workingSet
}

func (workingSet WorkingSet) ToDb() db.WorkingSet {
	dbServers := make(db.ServerList, len(workingSet.Servers))
	for i, server := range workingSet.Servers {
		dbServers[i] = db.Server{
			Type:    string(server.Type),
			Config:  server.Config,
			Secrets: server.Secrets,
			Tools:   server.Tools,
		}
		if server.Type == ServerTypeRegistry {
			dbServers[i].Source = server.Source
		}
		if server.Type == ServerTypeImage {
			dbServers[i].Image = server.Image
		}
		if server.Type == ServerTypeRemote {
			dbServers[i].Endpoint = server.Endpoint
		}
		if server.Snapshot != nil {
			dbServers[i].Snapshot = &db.ServerSnapshot{
				Server: server.Snapshot.Server,
			}
		}
	}

	dbSecrets := make(db.SecretMap, len(workingSet.Secrets))
	for name, secret := range workingSet.Secrets {
		dbSecrets[name] = db.Secret{
			Provider: string(secret.Provider),
		}
	}

	dbSet := db.WorkingSet{
		ID:      workingSet.ID,
		Name:    workingSet.Name,
		Servers: dbServers,
		Secrets: dbSecrets,
	}

	return dbSet
}

func (workingSet *WorkingSet) Validate() error {
	if err := validate.Get().Struct(workingSet); err != nil {
		return err
	}
	if err := workingSet.validateUniqueServerNames(); err != nil {
		return err
	}
	return workingSet.validateServerSnapshots()
}

func (workingSet *WorkingSet) validateUniqueServerNames() error {
	seen := make(map[string]bool)
	for _, server := range workingSet.Servers {
		// TODO: Update when Snapshot is required
		if server.Snapshot == nil {
			continue
		}
		name := server.Snapshot.Server.Name
		if seen[name] {
			return fmt.Errorf("duplicate server name %s", name)
		}
		seen[name] = true
	}
	return nil
}

func (workingSet *WorkingSet) validateServerSnapshots() error {
	for _, server := range workingSet.Servers {
		if err := server.Snapshot.ValidateInnerConfig(); err != nil {
			return err
		}
	}
	return nil
}

func (serverSnapshot *ServerSnapshot) ValidateInnerConfig() error {
	if serverSnapshot == nil {
		return nil
	}

	config := serverSnapshot.Server.Config
	if config == nil {
		return nil
	}

	for i, configItem := range config {
		configMap, ok := configItem.(map[string]any)
		if !ok {
			return fmt.Errorf("config[%d] is not a map", i)
		}

		_, ok = configMap["name"].(string)
		if !ok {
			return fmt.Errorf("config[%d] has no name field", i)
		}

		_, ok = configMap["description"].(string)
		if !ok {
			return fmt.Errorf("config[%d] has no description field", i)
		}

		t, ok := configMap["type"].(string)
		if !ok {
			return fmt.Errorf("config[%d] has no type field", i)
		}
		if t != "object" {
			return fmt.Errorf("config[%d].type must be 'object', got '%s'", i, t)
		}

		properties, ok := configMap["properties"].(map[string]any)
		if !ok {
			return fmt.Errorf("config[%d].properties is not a map", i)
		}

		if err := recursivePropertiesValidate(properties, fmt.Sprintf("config[%d].properties", i)); err != nil {
			return err
		}
	}

	return nil
}

func recursivePropertiesValidate(properties map[string]any, path string) error {
	for key, property := range properties {
		propertyPath := fmt.Sprintf("%s.%s", path, key)

		propertyMap, ok := property.(map[string]any)
		if !ok {
			return fmt.Errorf("%s is not a map", propertyPath)
		}

		t, ok := propertyMap["type"].(string)
		if !ok {
			return fmt.Errorf("%s has no type field", propertyPath)
		}

		switch t {
		case "string", "integer", "number", "boolean":
			continue
		case "object":
			innerProperties, ok := propertyMap["properties"].(map[string]any)
			if !ok {
				return fmt.Errorf("%s is type 'object' but has no properties field", propertyPath)
			}
			if err := recursivePropertiesValidate(innerProperties, propertyPath); err != nil {
				return err
			}
		case "array":
			items, ok := propertyMap["items"].(map[string]any)
			if !ok {
				return fmt.Errorf("%s is type 'array' but has no items field", propertyPath)
			}
			itemType, ok := items["type"].(string)
			if !ok {
				return fmt.Errorf("%s.items has no type field", propertyPath)
			}
			if itemType != "string" {
				return fmt.Errorf("%s.items type must be string", propertyPath)
			}
		default:
			return fmt.Errorf("%s.type %s is not supported", propertyPath, t)
		}
	}
	return nil
}

func (workingSet *WorkingSet) FindServer(serverName string) *Server {
	for i := range len(workingSet.Servers) {
		if workingSet.Servers[i].Snapshot == nil {
			// TODO(cody): Can happen with registry (for now)
			continue
		}
		if workingSet.Servers[i].Snapshot.Server.Name == serverName {
			return &workingSet.Servers[i]
		}
	}
	return nil
}

func (workingSet *WorkingSet) EnsureSnapshotsResolved(ctx context.Context, ociService oci.Service) error {
	// Ensure all snapshots are resolved
	for i := range len(workingSet.Servers) {
		if workingSet.Servers[i].Snapshot != nil {
			continue
		}
		log.Log(fmt.Sprintf("Server %s has no snapshot, lazy loading the snapshot...\n", workingSet.Servers[i].BasicName()))
		snapshot, err := ResolveSnapshot(ctx, ociService, workingSet.Servers[i])
		if err != nil {
			return fmt.Errorf("failed to resolve snapshot for server[%d]: %w", i, err)
		}
		// TODO(cody): Can be nil with registry (for now)
		if snapshot != nil {
			workingSet.Servers[i].Snapshot = snapshot
		}
	}

	return nil
}

func (s *Server) BasicName() string {
	switch s.Type {
	case ServerTypeImage:
		return s.Image
	case ServerTypeRegistry:
		return s.Source
	}
	return "unknown"
}

func createWorkingSetID(ctx context.Context, name string, dao db.DAO) (string, error) {
	// Replace all non-alphanumeric characters with a hyphen and make all uppercase lowercase
	re := regexp.MustCompile("[^a-zA-Z0-9]+")
	cleaned := re.ReplaceAllString(name, "-")
	baseName := strings.ToLower(cleaned)

	existingSets, err := dao.FindWorkingSetsByIDPrefix(ctx, baseName)
	if err != nil {
		return "", fmt.Errorf("failed to find profiles by name prefix: %w", err)
	}

	if len(existingSets) == 0 {
		return baseName, nil
	}

	takenIDs := make(map[string]bool)
	for _, set := range existingSets {
		takenIDs[set.ID] = true
	}

	// TODO(cody): there are better ways to do this, but this is a simple brute force for now
	// Append a number to the base name
	for i := 2; i <= 100; i++ {
		newName := fmt.Sprintf("%s-%d", baseName, i)
		if !takenIDs[newName] {
			return newName, nil
		}
	}

	return "", fmt.Errorf("failed to create profile id")
}

func ResolveServersFromString(ctx context.Context, registryClient registryapi.Client, ociService oci.Service, dao db.DAO, value string) ([]Server, error) {
	if v, ok := strings.CutPrefix(value, "docker://"); ok {
		fullRef, err := ResolveImageRef(ctx, ociService, v)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve image ref: %w", err)
		}
		serverSnapshot, err := ResolveImageSnapshot(ctx, ociService, fullRef)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve image snapshot: %w", err)
		}
		return []Server{{
			Type:     ServerTypeImage,
			Image:    fullRef,
			Secrets:  "default",
			Snapshot: serverSnapshot,
		}}, nil
	} else if v, ok := strings.CutPrefix(value, "catalog://"); ok {
		return ResolveCatalogServers(ctx, dao, v)
	} else if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") { // Assume registry entry if it's a URL
		server, err := ResolveRegistry(ctx, registryClient, value)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve registry: %w", err)
		}
		return []Server{server}, nil
	} else if v, ok := strings.CutPrefix(value, "file://"); ok {
		return ResolveFile(v)
	}
	return nil, fmt.Errorf("invalid server value: %s", value)
}

func ResolveFile(value string) ([]Server, error) {
	buf, err := os.ReadFile(value)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// First, see if it's a full legacy catalog file.
	// Fallback to a single server if it's not.
	var probe struct {
		Registry map[string]catalog.Server `yaml:"registry,omitempty" json:"registry,omitempty"`
	}

	var servers []catalog.Server
	switch filepath.Ext(strings.ToLower(value)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(buf, &probe); err != nil {
			return nil, fmt.Errorf("failed to unmarshal server: %w", err)
		}
		if probe.Registry == nil {
			// Fallback to parsing single server
			var server catalog.Server
			if err := yaml.Unmarshal(buf, &server); err != nil {
				return nil, fmt.Errorf("failed to unmarshal server: %w", err)
			}
			servers = []catalog.Server{server}
		}
	case ".json":
		if err := json.Unmarshal(buf, &probe); err != nil {
			return nil, fmt.Errorf("failed to unmarshal server: %w", err)
		}
		if probe.Registry == nil {
			// Fallback to parsing single server
			var server catalog.Server
			if err := json.Unmarshal(buf, &server); err != nil {
				return nil, fmt.Errorf("failed to unmarshal server: %w", err)
			}
			servers = []catalog.Server{server}
		}
	default:
		return nil, fmt.Errorf("unsupported file extension: %s, must be .yaml or .json", value)
	}

	if probe.Registry != nil {
		for name, server := range probe.Registry {
			server.Name = name
			servers = append(servers, server)
		}
	}

	serversResolved := make([]Server, len(servers))
	for i, server := range servers {
		if (server.Type == "server" || server.Type == "poci") && server.Image != "" {
			serversResolved[i] = Server{
				Type:     ServerTypeImage,
				Image:    server.Image,
				Secrets:  "default",
				Snapshot: &ServerSnapshot{Server: server},
			}
		} else if server.Type == "remote" {
			serversResolved[i] = Server{
				Type:     ServerTypeRemote,
				Endpoint: server.Remote.URL,
				Secrets:  "default",
				Snapshot: &ServerSnapshot{Server: server},
			}
		} else {
			return nil, fmt.Errorf("unsupported server type: %s", server.Type)
		}
	}

	return serversResolved, nil
}

func ResolveCatalogServers(ctx context.Context, dao db.DAO, value string) ([]Server, error) {
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid catalog URL: catalog://%s", value)
	}
	catalogRef := strings.Join(parts[:len(parts)-1], "/")
	serverList := parts[len(parts)-1]

	serverNames := strings.Split(serverList, "+")

	if len(serverNames) == 0 || len(serverList) == 0 {
		return nil, fmt.Errorf("no servers specified in catalog URL: catalog://%s", value)
	}

	ref, err := name.ParseReference(catalogRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse catalog reference %s: %w", catalogRef, err)
	}
	catalogRef = oci.FullNameWithoutDigest(ref)

	dbCatalog, err := dao.GetCatalog(ctx, catalogRef)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("catalog %s not found", catalogRef)
		}
		return nil, fmt.Errorf("failed to get catalog: %w", err)
	}

	filteredServers := make([]db.CatalogServer, 0, len(dbCatalog.Servers))
	foundPatterns := make(map[string]bool)
	foundServers := make(map[string]bool) // avoid duplicates
	for _, name := range serverNames {
		for _, server := range dbCatalog.Servers {
			// Glob support for selecting servers by name
			matched, err := path.Match(name, server.Snapshot.Server.Name)
			if err != nil {
				return nil, fmt.Errorf("bad pattern for catalog server '%s': %w", name, err)
			}
			if matched {
				if !foundServers[server.Snapshot.Server.Name] {
					// Only add to avoid duplicates
					filteredServers = append(filteredServers, server)
					foundServers[server.Snapshot.Server.Name] = true
				}
				foundPatterns[name] = true
			}
		}
	}

	uniqueServerNames := make(map[string]bool)
	for _, serverName := range serverNames {
		uniqueServerNames[serverName] = true
	}

	if len(foundPatterns) != len(uniqueServerNames) {
		f := make([]string, 0, len(foundPatterns))
		for pattern := range foundPatterns {
			f = append(f, pattern)
		}
		missingPatterns := sliceutil.Difference(serverNames, f)
		return nil, fmt.Errorf("servers matching the following patterns were not found in catalog: %v", missingPatterns)
	}

	return mapCatalogServersToWorkingSetServers(filteredServers, "default"), nil
}

func ResolveImageRef(ctx context.Context, ociService oci.Service, value string) (string, error) {
	ref, err := name.ParseReference(value)
	if err != nil {
		return "", fmt.Errorf("failed to parse reference: %w", err)
	}
	isRemote := false
	img, err := ociService.GetLocalImage(ctx, ref)
	if oci.IsNoSuchImageError(err) {
		img, err = ociService.GetRemoteImage(ctx, ref)
		isRemote = true
	}
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}
	var fullRef string
	if !isRemote || oci.HasDigest(ref) {
		// Local images shouldn't be referenced by a digest
		fullRef = ref.String()
	} else {
		// Remotes should be pinned to a digest
		digest, err := ociService.GetImageDigest(img)
		if err != nil {
			return "", fmt.Errorf("failed to get image digest: %w", err)
		}
		fullRef = fmt.Sprintf("%s@%s", ref.String(), digest)
	}

	return fullRef, nil
}

// ConvertRegistryServerToCatalog converts a community MCP registry server to Docker catalog format
// Only processes OCI packages (non-OCI packages are ignored)
// ConvertRegistryServerToCatalog converts a registry API server response to a catalog server.
// It extracts the first OCI package and converts runtime arguments, environment variables,
// secrets, and configuration items to the catalog format.
func ConvertRegistryServerToCatalog(serverResp *v0.ServerResponse) (catalog.Server, error) {
	server := serverResp.Server

	// Find first OCI package
	var ociPkg *model.Package
	for i := range server.Packages {
		if server.Packages[i].RegistryType == "oci" {
			ociPkg = &server.Packages[i]
			break
		}
	}

	if ociPkg == nil {
		return catalog.Server{}, fmt.Errorf("no OCI packages found for server")
	}

	catalogSrv := catalog.Server{
		Name:        NormalizeServerName(server.Name),
		Type:        "server",
		Image:       ociPkg.Identifier,
		Description: server.Description,
		Title:       server.Title,
	}

	// Extract icon (use first icon if available)
	if len(server.Icons) > 0 {
		catalogSrv.Icon = server.Icons[0].Src
	}

	// Build registry URL for server identity
	// Format: https://registry.modelcontextprotocol.io/v0/servers/{encoded_name}/versions/{version}
	if server.Name != "" && server.Version != "" {
		catalogSrv.Metadata = &catalog.Metadata{
			RegistryURL: registryapi.BuildServerURL(server.Name, server.Version),
		}
	}

	// Parse runtime arguments (volumes, user, and config for variables like workspace_path)
	var runtimeConfig []any
	catalogSrv.Volumes, catalogSrv.User, runtimeConfig = parseRuntimeArguments(catalogSrv.Name, ociPkg.RuntimeArguments)

	// Parse package arguments (command)
	catalogSrv.Command = parsePackageArguments(ociPkg.PackageArguments)

	// Process environment variables (secrets, env vars, config)
	catalogSrv.Secrets, catalogSrv.Env, catalogSrv.Config = processEnvironmentVariables(catalogSrv.Name, ociPkg.EnvironmentVariables)

	// Merge runtime argument config items (prepend so they appear first)
	if len(runtimeConfig) > 0 {
		catalogSrv.Config = append(runtimeConfig, catalogSrv.Config...)
	}

	return catalogSrv, nil
}

// parseRuntimeArguments extracts volumes, user settings, and config items from runtime arguments.
func parseRuntimeArguments(serverName string, args []model.Argument) (volumes []string, user string, config []any) {
	for _, arg := range args {
		if arg.Type != model.ArgumentTypeNamed || arg.Value == "" {
			continue
		}
		switch arg.Name {
		case "-v", "--volume":
			// Convert {var} placeholders to {{serverName.var}} format for eval.EvaluateList
			volumeValue := convertPlaceholders(serverName, arg.Value)
			volumes = append(volumes, volumeValue)
			// Extract variables as config items (e.g., workspace_path for volume mounts)
			if len(arg.Variables) > 0 {
				config = append(config, buildRuntimeArgConfigItem(serverName, arg))
			}
		case "-u", "--user":
			user = arg.Value
		}
	}
	return volumes, user, config
}

// convertPlaceholders converts registry-style {var} placeholders to catalog-style {{serverName.var}} format.
// Example: "{workspace_path}:/workspace" -> "{{arm-mcp.workspace_path}}:/workspace"
func convertPlaceholders(serverName, value string) string {
	// Match {var_name} but not {{var_name}} (already converted)
	re := regexp.MustCompile(`\{([^{}]+)\}`)
	return re.ReplaceAllString(value, "{{"+serverName+".$1}}")
}

// buildRuntimeArgConfigItem creates a config item from runtime argument variables.
// Uses the server name as the config name (e.g., "arm-mcp" instead of "-v").
func buildRuntimeArgConfigItem(serverName string, arg model.Argument) map[string]any {
	properties := make(map[string]any)
	var required []string

	for varName, varInput := range arg.Variables {
		prop := map[string]any{
			"type":        inferJSONType(string(varInput.Format)),
			"description": varInput.Description,
		}
		if varInput.Default != "" {
			prop["default"] = varInput.Default
		}
		if varInput.Placeholder != "" {
			prop["placeholder"] = varInput.Placeholder
		}
		if len(varInput.Choices) > 0 {
			prop["enum"] = varInput.Choices
		}
		properties[varName] = prop

		if varInput.IsRequired {
			required = append(required, varName)
		}
	}

	// Use just the server name part (after /) for config name
	// e.g., "arm/arm-mcp" -> "arm-mcp" to match Docker catalog format
	configName := serverName
	if idx := strings.LastIndex(serverName, "/"); idx != -1 {
		configName = serverName[idx+1:]
	}

	configItem := map[string]any{
		"name":        configName,
		"description": arg.Description,
		"type":        "object",
		"properties":  properties,
	}
	if len(required) > 0 {
		configItem["required"] = required
	}
	return configItem
}

// parsePackageArguments converts package arguments to a command array.
func parsePackageArguments(args []model.Argument) []string {
	var command []string
	for _, arg := range args {
		if arg.Value != "" {
			command = append(command, arg.Value)
		}
	}
	return command
}

// processEnvironmentVariables separates environment variables into secrets, env vars, and config items.
// All config properties are merged into a single config item named after the server (servername.field format).
func processEnvironmentVariables(serverName string, envVars []model.KeyValueInput) ([]catalog.Secret, []catalog.Env, []any) {
	var secrets []catalog.Secret
	var env []catalog.Env
	configProperties := make(map[string]any)
	var configRequired []string

	for _, envVar := range envVars {
		if envVar.IsSecret {
			secrets = append(secrets, catalog.Secret{
				Name: strings.ToLower(envVar.Name),
				Env:  envVar.Name,
			})
			continue
		}

		if !envVar.IsRequired && envVar.Default == "" && envVar.Value == "" {
			continue
		}

		// Complex config with nested variables - merge all variables into properties
		// AND create an Env entry with converted placeholders
		if len(envVar.Variables) > 0 {
			for varName, varInput := range envVar.Variables {
				prop := map[string]any{
					"type":        inferJSONType(string(varInput.Format)),
					"description": varInput.Description,
				}
				if varInput.Default != "" {
					prop["default"] = varInput.Default
				}
				if varInput.Placeholder != "" {
					prop["placeholder"] = varInput.Placeholder
				}
				if len(varInput.Choices) > 0 {
					prop["enum"] = varInput.Choices
				}
				configProperties[varName] = prop
				if varInput.IsRequired {
					configRequired = append(configRequired, varName)
				}
			}
			// Also create Env entry with converted placeholders so gateway exports it
			// Convert {var} to {{serverName.var}} format for eval.Evaluate
			env = append(env, catalog.Env{
				Name:  envVar.Name,
				Value: convertPlaceholders(serverName, envVar.Value),
			})
			continue
		}

		// Simple environment variable
		env = append(env, catalog.Env{
			Name:  envVar.Name,
			Value: envVar.Value,
		})

		// Add to config if it needs user input
		if envVar.Value == "" || strings.Contains(envVar.Value, "{") {
			lowerName := strings.ToLower(envVar.Name)
			prop := map[string]any{
				"type":        inferJSONType(string(envVar.Format)),
				"description": envVar.Description,
			}
			if envVar.Default != "" {
				prop["default"] = envVar.Default
			}
			configProperties[lowerName] = prop
			if envVar.IsRequired {
				configRequired = append(configRequired, lowerName)
			}
		}
	}

	// Build single config item with server name if we have any properties
	var config []any
	if len(configProperties) > 0 {
		configItem := map[string]any{
			"name":        serverName,
			"description": "Configuration for " + serverName,
			"type":        "object",
			"properties":  configProperties,
		}
		if len(configRequired) > 0 {
			configItem["required"] = configRequired
		}
		config = append(config, configItem)
	}

	return secrets, env, config
}

// NormalizeServerName converts a registry server name to namespace/server format
// Example: io.github.kubeshop/testkube-mcp -> kubeshop/testkube-mcp
func NormalizeServerName(name string) string {
	// Extract just the server name (after the last "/")
	// io.github.arm/arm-mcp -> arm-mcp
	// com.example.acme/my-server -> my-server
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		return name[idx+1:]
	}
	return name
}

// inferJSONType converts registry format to JSON schema type
func inferJSONType(format string) string {
	switch format {
	case "number":
		return "number"
	case "boolean":
		return "boolean"
	case "filepath":
		return "string"
	default:
		return "string"
	}
}

func ResolveRegistry(ctx context.Context, registryClient registryapi.Client, value string) (Server, error) {
	url, err := registryapi.ParseServerURL(value)
	if err != nil {
		return Server{}, fmt.Errorf("failed to parse server URL %s: %w", value, err)
	}

	versions, err := registryClient.GetServerVersions(ctx, url)
	if err != nil {
		return Server{}, fmt.Errorf("failed to get server versions from URL %s: %w", url.VersionsListURL(), err)
	}

	if len(versions.Servers) == 0 {
		return Server{}, fmt.Errorf("no server versions found for URL %s", url.VersionsListURL())
	}

	if url.IsLatestVersion() {
		latestVersion, err := resolveLatestVersion(versions)
		if err != nil {
			return Server{}, fmt.Errorf("failed to resolve latest version for server %s: %w", url.VersionsListURL(), err)
		}
		url = url.WithVersion(latestVersion)
	}

	var serverResp *v0.ServerResponse
	for _, version := range versions.Servers {
		if version.Server.Version == url.Version {
			serverResp = &version
			break
		}
	}
	if serverResp == nil {
		return Server{}, fmt.Errorf("server version not found")
	}

	// Check for OCI packages and convert to catalog format
	catalogServer, err := ConvertRegistryServerToCatalog(serverResp)
	if err != nil {
		return Server{}, fmt.Errorf("failed to convert registry server: %w", err)
	}

	return Server{
		Type:     ServerTypeRegistry,
		Source:   url.String(),
		Secrets:  "default",
		Snapshot: &ServerSnapshot{Server: catalogServer},
	}, nil
}

func ResolveSnapshot(ctx context.Context, ociService oci.Service, server Server) (*ServerSnapshot, error) {
	switch server.Type {
	case ServerTypeImage:
		return ResolveImageSnapshot(ctx, ociService, server.Image)
	case ServerTypeRegistry:
		// Snapshots for registry servers are resolved during ResolveRegistry
		return nil, nil //nolint:nilnil
	case ServerTypeRemote:
		// TODO(bobby): add snapshot when you can add remotes directly from URL
		return nil, nil //nolint:nilnil
	}
	return nil, fmt.Errorf("unsupported server type: %s", server.Type)
}

func ResolveImageSnapshot(ctx context.Context, ociService oci.Service, image string) (*ServerSnapshot, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	var img v1.Image
	// Anything with a digest should be a remote image
	if oci.HasDigest(ref) {
		img, err = ociService.GetRemoteImage(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to get remote image: %w", err)
		}
	} else {
		img, err = ociService.GetLocalImage(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to get local image: %w", err)
		}
	}

	serverSnapshot, err := getCatalogServerFromImage(ociService, img, image)
	if err != nil {
		return nil, fmt.Errorf("failed to get catalog server from image: %w", err)
	}
	return &ServerSnapshot{
		Server: serverSnapshot,
	}, nil
}

// Pins the "latest" to a specific version
func resolveLatestVersion(versions v0.ServerListResponse) (string, error) {
	for _, version := range versions.Servers {
		if version.Meta.Official.IsLatest {
			return version.Server.Version, nil
		}
	}
	return "", fmt.Errorf("no latest version found")
}

func getCatalogServerFromImage(ociService oci.Service, img v1.Image, name string) (catalog.Server, error) {
	labels, err := ociService.GetImageLabels(img)
	if err != nil {
		return catalog.Server{}, fmt.Errorf("failed to get image labels: %w", err)
	}
	metadataLabel := labels["io.docker.server.metadata"]
	if metadataLabel == "" {
		return catalog.Server{}, fmt.Errorf("image %s is not a self-describing image", name)
	}

	// Basic parsing validation
	var server catalog.Server
	if err := yaml.Unmarshal([]byte(metadataLabel), &server); err != nil {
		return catalog.Server{}, fmt.Errorf("failed to parse metadata label for %s: %w", name, err)
	}

	server.Type = "server"
	server.Image = name

	return server, nil
}

func mapCatalogServersToWorkingSetServers(dbServers []db.CatalogServer, secrets string) []Server {
	servers := make([]Server, len(dbServers))
	for i, server := range dbServers {
		servers[i] = Server{
			Type:     ServerType(server.ServerType),
			Tools:    ToolList(server.Tools),
			Config:   map[string]any{},
			Source:   server.Source,
			Image:    server.Image,
			Endpoint: server.Endpoint,
			Snapshot: &ServerSnapshot{
				Server: server.Snapshot.Server,
			},
			Secrets: secrets,
		}
	}
	return servers
}
