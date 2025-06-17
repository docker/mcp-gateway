package gateway

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/catalog"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/config"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/docker"
)

type Configurator interface {
	Read(ctx context.Context) (Configuration, chan Configuration, func() error, error)
}

type Configuration struct {
	serverNames []string
	servers     map[string]catalog.Server
	config      map[string]map[string]any
	secrets     map[string]string
}

type ServerConfig struct {
	Name    string
	Spec    catalog.Server
	Config  map[string]any
	Secrets map[string]string
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

func (c *Configuration) Find(serverName string) (*ServerConfig, *map[string]catalog.Tool, bool) {
	// Is it in the catalog?
	server, ok := c.servers[serverName]
	if !ok {
		return nil, nil, false
	}

	// Is it an MCP Server?
	if server.Image != "" || server.SSEEndpoint != "" {
		return &ServerConfig{
			Name: serverName,
			Spec: server,
			Config: map[string]any{
				serverName: c.config[serverName],
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
	CatalogPath  string
	ServerNames  []string // Takes precedence over the RegistryPath
	RegistryPath string
	ConfigPath   string
	SecretsPath  string // Optional, if not set, use Docker Desktop's secrets API
	Watch        bool

	docker docker.Client
}

func (c *FileBasedConfiguration) Read(ctx context.Context) (Configuration, chan Configuration, func() error, error) {
	configuration, err := c.readOnce(ctx)
	if err != nil {
		return Configuration{}, nil, nil, err
	}
	if !c.Watch {
		return configuration, nil, func() error { return nil }, nil
	}

	var registryPath string
	if len(c.ServerNames) == 0 {
		registryPath, err = config.FilePath(c.RegistryPath)
		if err != nil {
			return Configuration{}, nil, nil, err
		}
	}

	configPath, err := config.FilePath(c.ConfigPath)
	if err != nil {
		return Configuration{}, nil, nil, err
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

				if configuration, err := c.readOnce(ctx); err == nil {
					updates <- configuration
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logf("watch error: %s", err)
			}
		}
	}()

	if registryPath != "" {
		log("- Watching registry at", registryPath)
		if err := watcher.Add(registryPath); err != nil {
			_ = watcher.Close()
			return Configuration{}, nil, nil, err
		}
	}
	if configPath != "" {
		log("- Watching config at", configPath)
		if err := watcher.Add(configPath); err != nil {
			_ = watcher.Close()
			return Configuration{}, nil, nil, err
		}
	}

	return configuration, updates, watcher.Close, nil
}

func (c *FileBasedConfiguration) readOnce(ctx context.Context) (Configuration, error) {
	start := time.Now()
	log("- Reading configuration...")

	var serverNames []string

	if len(c.ServerNames) > 0 {
		serverNames = c.ServerNames
	} else {
		registryConfig, err := c.readRegistry(ctx)
		if err != nil {
			return Configuration{}, fmt.Errorf("reading registry: %w", err)
		}

		serverNames = registryConfig.ServerNames()
	}

	mcpCatalog, err := c.readCatalog(ctx)
	if err != nil {
		return Configuration{}, fmt.Errorf("reading catalog: %w", err)
	}
	servers := mcpCatalog.Servers

	serversConfig, err := c.readConfig(ctx)
	if err != nil {
		return Configuration{}, fmt.Errorf("reading config: %w", err)
	}

	var secrets map[string]string
	if c.SecretsPath != "" || os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1" {
		var err error
		secrets, err = c.readSecretsFromFile(ctx, c.SecretsPath)
		if err != nil {
			return Configuration{}, err
		}
	} else {
		var err error
		secrets, err = c.readDockerDesktopSecrets(ctx, servers, serverNames)
		if err != nil {
			return Configuration{}, err
		}
	}

	log("- Configuration read in", time.Since(start))
	return Configuration{
		serverNames: serverNames,
		servers:     servers,
		config:      serversConfig,
		secrets:     secrets,
	}, nil
}

func (c *FileBasedConfiguration) readCatalog(ctx context.Context) (catalog.Catalog, error) {
	log("  - Reading catalog from", c.CatalogPath)
	return catalog.ReadFrom(ctx, c.CatalogPath)
}

func (c *FileBasedConfiguration) readRegistry(ctx context.Context) (config.Registry, error) {
	if c.RegistryPath == "" {
		return config.Registry{}, nil
	}

	log("  - Reading registry from", c.RegistryPath)
	yaml, err := config.ReadConfigFile(ctx, c.docker, c.RegistryPath)
	if err != nil {
		return config.Registry{}, fmt.Errorf("reading registry.yaml: %w", err)
	}

	cfg, err := config.ParseRegistryConfig(yaml)
	if err != nil {
		return config.Registry{}, fmt.Errorf("parsing registry.yaml: %w", err)
	}

	return cfg, nil
}

func (c *FileBasedConfiguration) readConfig(ctx context.Context) (map[string]map[string]any, error) {
	if c.ConfigPath == "" {
		return map[string]map[string]any{}, nil
	}

	log("  - Reading config from", c.ConfigPath)
	yaml, err := config.ReadConfigFile(ctx, c.docker, c.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("reading config.yaml: %w", err)
	}

	cfg, err := config.ParseConfig(yaml)
	if err != nil {
		return nil, fmt.Errorf("parsing config.yaml: %w", err)
	}

	return cfg, nil
}

func (c *FileBasedConfiguration) readDockerDesktopSecrets(ctx context.Context, servers map[string]catalog.Server, serverNames []string) (map[string]string, error) {
	var secretNames []string
	for _, serverName := range serverNames {
		serverSpec, ok := servers[serverName]
		if !ok {
			continue
		}

		for _, s := range serverSpec.Secrets {
			secretNames = append(secretNames, s.Name)
		}
	}

	log("  - Reading secrets", secretNames)
	if len(secretNames) == 0 {
		return map[string]string{}, nil
	}

	secretsByName, err := secretValues(ctx, secretNames)
	if err != nil {
		return nil, fmt.Errorf("finding secrets %s: %w", secretNames, err)
	}

	return secretsByName, nil
}

func (c *FileBasedConfiguration) readSecretsFromFile(ctx context.Context, path string) (map[string]string, error) {
	if path == "" {
		return map[string]string{}, nil // No secrets file provided
	}

	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading secrets from %s: %w", path, err)
	}

	secrets := map[string]string{}

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
