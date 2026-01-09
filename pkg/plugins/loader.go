package plugins

import (
	"context"
	"fmt"
	"sync"
)

// Loader manages plugin discovery, loading, and lifecycle.
type Loader struct {
	mu      sync.RWMutex
	plugins map[string]*loadedPlugin
	hooks   *PluginLifecycleHooks
}

// loadedPlugin wraps a plugin with its transport and client.
type loadedPlugin struct {
	config    PluginConfig
	transport PluginTransport
	client    PluginClient
	info      PluginInfo
}

// NewLoader creates a new plugin loader.
func NewLoader(hooks *PluginLifecycleHooks) *Loader {
	return &Loader{
		plugins: make(map[string]*loadedPlugin),
		hooks:   hooks,
	}
}

// Load loads a plugin based on its configuration.
func (l *Loader) Load(ctx context.Context, config PluginConfig) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if already loaded
	if _, exists := l.plugins[config.Name]; exists {
		return fmt.Errorf("plugin %s is already loaded", config.Name)
	}

	// Create transport based on type
	transport, err := l.createTransport(config)
	if err != nil {
		return fmt.Errorf("failed to create transport for plugin %s: %w", config.Name, err)
	}

	// Connect to plugin
	client, err := transport.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to plugin %s: %w", config.Name, err)
	}

	// Create plugin info
	info := PluginInfo{
		Name:    config.Name,
		Type:    config.Type,
		Version: "unknown", // Could be fetched from plugin
	}

	// Store loaded plugin
	l.plugins[config.Name] = &loadedPlugin{
		config:    config,
		transport: transport,
		client:    client,
		info:      info,
	}

	// Call lifecycle hook
	if l.hooks != nil && l.hooks.OnStart != nil {
		l.hooks.OnStart(info)
	}

	return nil
}

// createTransport creates the appropriate transport for the plugin configuration.
func (l *Loader) createTransport(config PluginConfig) (PluginTransport, error) {
	switch config.Type {
	case "subprocess":
		if config.Subprocess == nil {
			return nil, fmt.Errorf("subprocess configuration is required")
		}
		return NewSubprocessTransport(*config.Subprocess, l.hooks), nil

	case "sidecar":
		if config.Sidecar == nil {
			return nil, fmt.Errorf("sidecar configuration is required")
		}
		return NewSidecarTransport(*config.Sidecar, l.hooks), nil

	default:
		return nil, fmt.Errorf("unknown plugin type: %s", config.Type)
	}
}

// LoadAll loads multiple plugins from configuration.
func (l *Loader) LoadAll(ctx context.Context, configs []PluginConfig) error {
	for _, config := range configs {
		if err := l.Load(ctx, config); err != nil {
			return err
		}
	}
	return nil
}

// Get returns a loaded plugin by name.
func (l *Loader) Get(name string) (PluginClient, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	plugin, exists := l.plugins[name]
	if !exists {
		return nil, false
	}
	return plugin.client, true
}

// GetInfo returns information about a loaded plugin.
func (l *Loader) GetInfo(name string) (PluginInfo, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	plugin, exists := l.plugins[name]
	if !exists {
		return PluginInfo{}, false
	}
	return plugin.info, true
}

// List returns information about all loaded plugins.
func (l *Loader) List() []PluginInfo {
	l.mu.RLock()
	defer l.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(l.plugins))
	for _, plugin := range l.plugins {
		infos = append(infos, plugin.info)
	}
	return infos
}

// Unload stops and removes a plugin.
func (l *Loader) Unload(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	plugin, exists := l.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s is not loaded", name)
	}

	// Close the client
	err := plugin.client.Close()

	// Remove from map
	delete(l.plugins, name)

	// Call lifecycle hook
	if l.hooks != nil && l.hooks.OnStop != nil {
		l.hooks.OnStop(plugin.info, err)
	}

	return err
}

// UnloadAll stops and removes all plugins.
func (l *Loader) UnloadAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var firstErr error
	for name, plugin := range l.plugins {
		err := plugin.client.Close()
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to unload plugin %s: %w", name, err)
		}

		// Call lifecycle hook
		if l.hooks != nil && l.hooks.OnStop != nil {
			l.hooks.OnStop(plugin.info, err)
		}
	}

	// Clear all plugins
	l.plugins = make(map[string]*loadedPlugin)

	return firstErr
}

// Reload reloads a plugin by unloading and loading it again.
func (l *Loader) Reload(ctx context.Context, name string) error {
	l.mu.Lock()
	plugin, exists := l.plugins[name]
	if !exists {
		l.mu.Unlock()
		return fmt.Errorf("plugin %s is not loaded", name)
	}
	config := plugin.config
	l.mu.Unlock()

	// Unload first
	if err := l.Unload(name); err != nil {
		return fmt.Errorf("failed to unload plugin %s: %w", name, err)
	}

	// Load again
	if err := l.Load(ctx, config); err != nil {
		return fmt.Errorf("failed to reload plugin %s: %w", name, err)
	}

	// Call lifecycle hook
	if l.hooks != nil && l.hooks.OnRestart != nil {
		l.hooks.OnRestart(plugin.info, 1)
	}

	return nil
}

// PluginsConfig represents the plugins section of a configuration file.
type PluginsConfig struct {
	Plugins map[string]PluginConfig `json:"plugins" yaml:"plugins"`
}

// LoadFromConfig loads plugins from a configuration structure.
// This is typically used when reading from a config file.
func (l *Loader) LoadFromConfig(ctx context.Context, config PluginsConfig) error {
	for name, pluginConfig := range config.Plugins {
		// Set the name if not already set
		if pluginConfig.Name == "" {
			pluginConfig.Name = name
		}
		if err := l.Load(ctx, pluginConfig); err != nil {
			return fmt.Errorf("failed to load plugin %s: %w", name, err)
		}
	}
	return nil
}
