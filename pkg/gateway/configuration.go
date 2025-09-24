package gateway

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/oci"
)

type Configurator interface {
	Read(ctx context.Context) (Configuration, chan Configuration, func() error, error)
}

type Configuration struct {
	serverNames []string
	servers     map[string]catalog.Server
	config      map[string]map[string]any
	tools       config.ToolsConfig
	secrets     map[string]string
}

func (c *Configuration) ServerNames() []string {
	return c.serverNames
}

func (c *Configuration) DockerImages() []string {
	uniqueDockerImages := map[string]bool{}

	for _, serverName := range c.serverNames {
		serverConfig, tools, found := c.Find(serverName)

		switch {
		case !found:
			log("MCP server not found:", serverName)
		case serverConfig != nil && serverConfig.Spec.Image != "":
			uniqueDockerImages[serverConfig.Spec.Image] = true
		case tools != nil:
			for _, tool := range *tools {
				uniqueDockerImages[tool.Container.Image] = true
			}
		}
	}

	var dockerImages []string
	for dockerImage := range uniqueDockerImages {
		dockerImages = append(dockerImages, dockerImage)
	}
	sort.Strings(dockerImages)
	return dockerImages
}

func (c *Configuration) Find(serverName string) (*catalog.ServerConfig, *map[string]catalog.Tool, bool) {
	serverName = strings.TrimSpace(serverName)

	// Is it in the catalog?
	server, ok := c.servers[serverName]
	if !ok {
		return nil, nil, false
	}

	// Is it an MCP Server?
	if server.Image != "" || server.SSEEndpoint != "" || server.Remote.URL != "" {
		return &catalog.ServerConfig{
			Name: serverName,
			Spec: server,
			Config: map[string]any{
				oci.CanonicalizeServerName(serverName): c.config[oci.CanonicalizeServerName(serverName)],
			},
			Secrets: c.secrets, // TODO: we could keep just the secrets for this server
		}, nil, true
	}

	// Then it's a POCI?
	byName := map[string]catalog.Tool{}
	for _, tool := range server.Tools {
		byName[tool.Name] = tool
	}
	return nil, &byName, true
}

type FileBasedConfiguration struct {
	CatalogPath        []string
	ServerNames        []string // Takes precedence over the RegistryPath
	RegistryPath       []string
	ConfigPath         []string
	ToolsPath          []string
	SecretsPath        string           // Optional, if not set, use Docker Desktop's secrets API
	OciRef             []string         // OCI references to fetch server definitions from
	MCPRegistryServers []catalog.Server // Servers fetched from MCP registries
	Watch              bool
	Central            bool
	McpOAuthDcrEnabled bool

	docker docker.Client
}

// isDCRFeatureEnabled checks if the mcp-oauth-dcr feature is enabled
func (c *FileBasedConfiguration) isDCRFeatureEnabled() bool {
	return c.McpOAuthDcrEnabled
}

func (c *FileBasedConfiguration) Read(ctx context.Context) (Configuration, chan Configuration, func() error, error) {
	configuration, err := c.readOnce(ctx)
	if err != nil {
		return Configuration{}, nil, nil, err
	}
	if !c.Watch {
		return configuration, nil, func() error { return nil }, nil
	}

	var registryPaths []string
	if len(c.ServerNames) == 0 {
		for _, path := range c.RegistryPath {
			if path != "" {
				registryPath, err := config.FilePath(path)
				if err != nil {
					return Configuration{}, nil, nil, err
				}
				registryPaths = append(registryPaths, registryPath)
			}
		}
	}

	var configPaths []string
	for _, path := range c.ConfigPath {
		if path != "" {
			configPath, err := config.FilePath(path)
			if err != nil {
				return Configuration{}, nil, nil, err
			}
			configPaths = append(configPaths, configPath)
		}
	}

	var toolsPaths []string
	for _, path := range c.ToolsPath {
		if path != "" {
			toolsPath, err := config.FilePath(path)
			if err != nil {
				return Configuration{}, nil, nil, err
			}
			toolsPaths = append(toolsPaths, toolsPath)
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return Configuration{}, nil, nil, err
	}

	updates := make(chan Configuration)
	go func() {
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Debounce: drain any additional events to avoid rapid reloads
			debounce:
				for {
					select {
					case <-time.After(300 * time.Millisecond):
						break debounce
					case <-watcher.Events:
					}
				}

				configuration, err := c.readOnce(ctx)
				if err != nil {
					log("Error reading configuration:", err)
					continue
				}

				updates <- configuration

			case <-ctx.Done():
				return
			}
		}
	}()

	// Add all registry paths to watcher
	for _, path := range registryPaths {
		if err := watcher.Add(path); err != nil && !os.IsNotExist(err) {
			return Configuration{}, nil, nil, err
		}
	}

	// Add all config paths to watcher
	for _, path := range configPaths {
		if err := watcher.Add(path); err != nil && !os.IsNotExist(err) {
			return Configuration{}, nil, nil, err
		}
	}

	// Add all tools paths to watcher
	for _, path := range toolsPaths {
		if err := watcher.Add(path); err != nil && !os.IsNotExist(err) {
			return Configuration{}, nil, nil, err
		}
	}

	// Add token event file to watcher only if DCR feature is enabled
	if c.isDCRFeatureEnabled() {
		tokenEventPath := filepath.Join(os.Getenv("HOME"), ".docker", "mcp", TokenEventFilename)
		if err := watcher.Add(tokenEventPath); err != nil && !os.IsNotExist(err) {
			log(fmt.Sprintf("DCR: Warning - Could not watch token event file %s: %v", tokenEventPath, err))
			// Don't fail configuration loading if token event file can't be watched
		} else {
			log(fmt.Sprintf("DCR: Watching token event file: %s", tokenEventPath))
		}
	} else {
		log("DCR: Token event file watching disabled (mcp-oauth-dcr feature inactive)")
	}

	return configuration, updates, watcher.Close, nil
}

func (c *FileBasedConfiguration) readOnce(ctx context.Context) (Configuration, error) {
	start := time.Now()
	log("- Reading configuration...")

	var serverNames []string
	if !c.Central {
		if len(c.ServerNames) > 0 {
			serverNames = c.ServerNames
		} else {
			registryConfig, err := c.readRegistry(ctx)
			if err != nil {
				return Configuration{}, fmt.Errorf("reading registry: %w", err)
			}

			serverNames = registryConfig.ServerNames()
		}
	}

	// check for docker.io self-contained servers
	selfContainedCatalog, updatedServerNames, err := oci.SelfContainedCatalog(ctx, c.docker, serverNames)
	if err != nil {
		return Configuration{}, fmt.Errorf("reading oci server: %w", err)
	}
	serverNames = updatedServerNames

	// read local caalog files
	mcpCatalog, err := c.readCatalog(ctx)
	if err != nil {
		return Configuration{}, fmt.Errorf("reading catalog: %w", err)
	}

	// merge self-contained servers with local catalog
	servers := mcpCatalog.Servers
	for k, v := range selfContainedCatalog.Servers {
		servers[k] = v
	}

	// Read servers from OCI references if any are provided
	ociServers, err := c.readServersFromOci(ctx)
	if err != nil {
		return Configuration{}, fmt.Errorf("reading servers from OCI: %w", err)
	}

	// Merge OCI servers into the main servers map and add to serverNames list
	for serverName, server := range ociServers {
		if _, exists := servers[serverName]; exists {
			log(fmt.Sprintf("Warning: server '%s' from OCI reference overwrites server from catalog", serverName))
		}
		servers[serverName] = server

		// Add to serverNames list if not already present
		found := false
		for _, existing := range serverNames {
			if existing == serverName {
				found = true
				break
			}
		}
		if !found {
			serverNames = append(serverNames, serverName)
		}
	}

	// Add MCP registry servers if any are provided
	if len(c.MCPRegistryServers) > 0 {
		for i, mcpServer := range c.MCPRegistryServers {
			// Generate a unique name for the MCP registry server based on its image
			serverName := fmt.Sprintf("mcp-registry-%d", i)
			if mcpServer.Image != "" {
				// Use image name as server name if available, cleaned up
				parts := strings.Split(mcpServer.Image, "/")
				imageName := parts[len(parts)-1] // Get the last part (image:tag)
				if colonIdx := strings.Index(imageName, ":"); colonIdx != -1 {
					imageName = imageName[:colonIdx] // Remove tag
				}
				serverName = fmt.Sprintf("mcp-registry-%s", imageName)
			}

			// Ensure unique server name
			originalName := serverName
			counter := 1
			for _, exists := servers[serverName]; exists; _, exists = servers[serverName] {
				serverName = fmt.Sprintf("%s-%d", originalName, counter)
				counter++
			}

			// Add the MCP registry server directly
			servers[serverName] = mcpServer

			// Add to serverNames list if not already present
			found := false
			for _, existing := range serverNames {
				if existing == serverName {
					found = true
					break
				}
			}
			if !found {
				serverNames = append(serverNames, serverName)
			}

			log(fmt.Sprintf("Added MCP registry server: %s (image: %s)", serverName, mcpServer.Image))
		}
	}

	// TODO(dga): Do we expect every server to have a config, in Central mode?
	serversConfig, err := c.readConfig(ctx)
	if err != nil {
		return Configuration{}, fmt.Errorf("reading config: %w", err)
	}

	serverToolsConfig, err := c.readToolsConfig(ctx)
	if err != nil {
		return Configuration{}, fmt.Errorf("reading tools: %w", err)
	}

	// TODO(dga): How do we know which secrets to read, in Central mode?
	var secrets map[string]string
	if c.SecretsPath == "docker-desktop" {
		secrets, err = c.readDockerDesktopSecrets(ctx, servers, serverNames)
		if err != nil {
			return Configuration{}, fmt.Errorf("reading MCP Toolkit's secrets: %w", err)
		}
	} else {
		// Unless SecretsPath is only `docker-desktop`, we don't fail if secrets can't be read.
		// It's ok for the MCP tookit's to not be available (in Cloud Run, for example).
		// It's ok for secrets .env file to not exist.
		var err error
		for secretPath := range strings.SplitSeq(c.SecretsPath, ":") {
			if secretPath == "docker-desktop" {
				secrets, err = c.readDockerDesktopSecrets(ctx, servers, serverNames)
			} else {
				secrets, err = c.readSecretsFromFile(ctx, secretPath)
			}

			if err == nil {
				break
			}
		}
	}

	log("- Configuration read in", time.Since(start))
	return Configuration{
		serverNames: serverNames,
		servers:     servers,
		config:      serversConfig,
		tools:       serverToolsConfig,
		secrets:     secrets,
	}, nil
}

func (c *FileBasedConfiguration) readCatalog(ctx context.Context) (catalog.Catalog, error) {
	log("  - Reading catalog from", c.CatalogPath)
	return catalog.ReadFrom(ctx, c.CatalogPath)
}

func (c *FileBasedConfiguration) readRegistry(ctx context.Context) (config.Registry, error) {
	if len(c.RegistryPath) == 0 {
		return config.Registry{}, nil
	}

	mergedRegistry := config.Registry{
		Servers: map[string]config.Tile{},
	}

	for _, registryPath := range c.RegistryPath {
		if registryPath == "" {
			continue
		}

		log("  - Reading registry from", registryPath)
		yaml, err := config.ReadConfigFile(ctx, c.docker, registryPath)
		if err != nil {
			return config.Registry{}, fmt.Errorf("reading registry file %s: %w", registryPath, err)
		}

		cfg, err := config.ParseRegistryConfig(yaml)
		if err != nil {
			return config.Registry{}, fmt.Errorf("parsing registry file %s: %w", registryPath, err)
		}

		// Merge servers into the combined registry, checking for overlaps
		for serverName, tile := range cfg.Servers {
			if _, exists := mergedRegistry.Servers[serverName]; exists {
				log(fmt.Sprintf("Warning: overlapping server '%s' found in registry '%s', overwriting previous value", serverName, registryPath))
			}
			mergedRegistry.Servers[serverName] = tile
		}
	}

	return mergedRegistry, nil
}

func (c *FileBasedConfiguration) readConfig(ctx context.Context) (map[string]map[string]any, error) {
	if len(c.ConfigPath) == 0 {
		return map[string]map[string]any{}, nil
	}

	mergedConfig := map[string]map[string]any{}

	for _, configPath := range c.ConfigPath {
		if configPath == "" {
			continue
		}

		log("  - Reading config from", configPath)
		yaml, err := config.ReadConfigFile(ctx, c.docker, configPath)
		if err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
		}

		cfg, err := config.ParseConfig(yaml)
		if err != nil {
			return nil, fmt.Errorf("parsing config file %s: %w", configPath, err)
		}

		// Merge configs into the combined config, checking for overlaps
		for serverName, serverConfig := range cfg {
			if _, exists := mergedConfig[serverName]; exists {
				log(fmt.Sprintf("Warning: overlapping server config '%s' found in config file '%s', overwriting previous value", serverName, configPath))
			}
			mergedConfig[serverName] = serverConfig
		}
	}

	return mergedConfig, nil
}

func (c *FileBasedConfiguration) readToolsConfig(ctx context.Context) (config.ToolsConfig, error) {
	if len(c.ToolsPath) == 0 {
		return config.ToolsConfig{}, nil
	}

	mergedToolsConfig := config.ToolsConfig{
		ServerTools: make(map[string][]string),
	}

	for _, toolsPath := range c.ToolsPath {
		if toolsPath == "" {
			continue
		}

		log("  - Reading tools from", toolsPath)
		yaml, err := config.ReadConfigFile(ctx, c.docker, toolsPath)
		if err != nil {
			return config.ToolsConfig{}, fmt.Errorf("reading tools file %s: %w", toolsPath, err)
		}

		toolsConfig, err := config.ParseToolsConfig(yaml)
		if err != nil {
			return config.ToolsConfig{}, fmt.Errorf("parsing tools file %s: %w", toolsPath, err)
		}

		// Merge tools into the combined tools, checking for overlaps
		for serverName, serverTools := range toolsConfig.ServerTools {
			if _, exists := mergedToolsConfig.ServerTools[serverName]; exists {
				log(fmt.Sprintf("Warning: overlapping server tools '%s' found in tools file '%s', overwriting previous value", serverName, toolsPath))
			}
			mergedToolsConfig.ServerTools[serverName] = serverTools
		}
	}

	return mergedToolsConfig, nil
}

func (c *FileBasedConfiguration) readDockerDesktopSecrets(ctx context.Context, servers map[string]catalog.Server, serverNames []string) (map[string]string, error) {
	// Use a map to deduplicate secret names
	uniqueSecretNames := make(map[string]struct{})

	for _, serverName := range serverNames {
		serverName := strings.TrimSpace(serverName)

		serverSpec, ok := servers[serverName]
		if !ok {
			continue
		}

		for _, s := range serverSpec.Secrets {
			uniqueSecretNames[s.Name] = struct{}{}
		}
	}

	if len(uniqueSecretNames) == 0 {
		return map[string]string{}, nil
	}

	// Convert map keys to slice
	var secretNames []string
	for name := range uniqueSecretNames {
		secretNames = append(secretNames, name)
	}

	log("  - Reading secrets", secretNames)
	secretsByName, err := c.docker.ReadSecrets(ctx, secretNames, true)
	if err != nil {
		return nil, fmt.Errorf("finding secrets %s: %w", secretNames, err)
	}

	return secretsByName, nil
}

func (c *FileBasedConfiguration) readSecretsFromFile(ctx context.Context, path string) (map[string]string, error) {
	secrets := map[string]string{}

	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading secrets from %s: %w", path, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var key, value string
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid line in secrets file: %s", line)
		}

		secrets[key] = value
	}

	return secrets, nil
}

// readServersFromOci fetches and parses server definitions from OCI references
func (c *FileBasedConfiguration) readServersFromOci(_ context.Context) (map[string]catalog.Server, error) {
	ociServers := make(map[string]catalog.Server)

	if len(c.OciRef) == 0 {
		return ociServers, nil
	}

	log("  - Reading servers from OCI references", c.OciRef)

	for _, ociRef := range c.OciRef {
		if ociRef == "" {
			continue
		}

		// Use the existing oci.ReadArtifact function to get the Catalog data
		ociCatalog, err := oci.ReadArtifact(ociRef)
		if err != nil {
			return nil, fmt.Errorf("failed to read OCI artifact %s: %w", ociRef, err)
		}

		// Process each server in the OCI catalog registry
		for i, ociServer := range ociCatalog.Registry {
			// The ServerDetail is now directly available in ociServer.Server
			serverDetail := ociServer.Server

			// Transform ServerDetail to catalog.Server using the ToCatalogServer method
			server := serverDetail.ToCatalogServer()

			// Use the name from the ServerDetail if available, otherwise generate one
			serverName := serverDetail.Name
			if serverName == "" {
				serverName = fmt.Sprintf("oci-server-%d", i)
			}

			if _, exists := ociServers[serverName]; exists {
				log(fmt.Sprintf("Warning: overlapping server '%s' found in OCI reference '%s', overwriting previous value", serverName, ociRef))
			}
			ociServers[serverName] = server
			log(fmt.Sprintf("  - Added server '%s' from OCI reference %s", serverName, ociRef))
		}
	}

	return ociServers, nil
}
