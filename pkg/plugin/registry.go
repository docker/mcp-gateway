package plugin

import (
	"fmt"
	"sync"
)

// Registry manages plugin registration and lifecycle.
// It provides a central place to register and retrieve plugins by type.
type Registry struct {
	mu              sync.RWMutex
	telemetryPlugin TelemetryPlugin
}

// globalRegistry is the default plugin registry instance.
var globalRegistry = &Registry{}

// Global returns the global plugin registry.
func Global() *Registry {
	return globalRegistry
}

// RegisterTelemetryPlugin registers a telemetry plugin.
// Only one telemetry plugin can be registered at a time.
// Returns an error if a plugin is already registered.
func (r *Registry) RegisterTelemetryPlugin(plugin TelemetryPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.telemetryPlugin != nil {
		return fmt.Errorf("telemetry plugin already registered")
	}
	r.telemetryPlugin = plugin
	return nil
}

// UnregisterTelemetryPlugin removes the registered telemetry plugin.
// It calls Close() on the plugin before removing it.
func (r *Registry) UnregisterTelemetryPlugin() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.telemetryPlugin != nil {
		if err := r.telemetryPlugin.Close(); err != nil {
			return fmt.Errorf("failed to close telemetry plugin: %w", err)
		}
		r.telemetryPlugin = nil
	}
	return nil
}

// TelemetryPlugin returns the registered telemetry plugin, or nil if none is registered.
func (r *Registry) TelemetryPlugin() TelemetryPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.telemetryPlugin
}

// HasTelemetryPlugin returns true if a telemetry plugin is registered.
func (r *Registry) HasTelemetryPlugin() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.telemetryPlugin != nil
}

// Close shuts down all registered plugins.
func (r *Registry) Close() error {
	return r.UnregisterTelemetryPlugin()
}

// ResetForTesting resets the registry state for testing purposes.
// This should only be used in tests.
func (r *Registry) ResetForTesting() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.telemetryPlugin = nil
}
