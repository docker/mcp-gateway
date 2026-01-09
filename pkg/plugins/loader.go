package plugins

import (
	"context"
	"fmt"
	"sync"
)

// PluginRegistry manages plugin providers and loaded plugin instances.
type PluginRegistry struct {
	mu        sync.RWMutex
	providers map[string]PluginProvider

	// Loaded plugin instances
	authProvider      AuthProvider
	credentialStorage CredentialStorage
	authProxy         AuthProxy
	auditSink         AuditSink
	policyEvaluator   PolicyEvaluator
	mcpProvisioner    MCPProvisioner
	telemetryPlugin   TelemetryPlugin
}

// globalRegistry is the default plugin registry instance.
var globalRegistry = &PluginRegistry{
	providers: make(map[string]PluginProvider),
}

// Global returns the global plugin registry.
func Global() *PluginRegistry {
	return globalRegistry
}

// RegisterProvider registers a plugin provider.
func (r *PluginRegistry) RegisterProvider(provider PluginProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

// GetProvider returns a registered provider by name.
func (r *PluginRegistry) GetProvider(name string) (PluginProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[name]
	return provider, ok
}

// LoadPlugins loads all plugins from configuration.
func (r *PluginRegistry) LoadPlugins(ctx context.Context, config PluginsConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load auth provider
	if config.AuthProvider != nil {
		provider, err := r.getProvider(config.AuthProvider.Provider)
		if err != nil {
			return fmt.Errorf("failed to load auth provider: %w", err)
		}
		r.authProvider, err = provider.CreateAuthProvider(ctx, *config.AuthProvider)
		if err != nil {
			return fmt.Errorf("failed to load auth provider: %w", err)
		}
	}

	// Load credential storage
	if config.CredentialStorage != nil {
		provider, err := r.getProvider(config.CredentialStorage.Provider)
		if err != nil {
			return fmt.Errorf("failed to load credential storage: %w", err)
		}
		r.credentialStorage, err = provider.CreateCredentialStorage(ctx, *config.CredentialStorage)
		if err != nil {
			return fmt.Errorf("failed to load credential storage: %w", err)
		}
	}

	// Load auth proxy
	if config.AuthProxy != nil {
		provider, err := r.getProvider(config.AuthProxy.Provider)
		if err != nil {
			return fmt.Errorf("failed to load auth proxy: %w", err)
		}
		r.authProxy, err = provider.CreateAuthProxy(ctx, *config.AuthProxy)
		if err != nil {
			return fmt.Errorf("failed to load auth proxy: %w", err)
		}
	}

	// Load audit sink
	if config.AuditSink != nil {
		provider, err := r.getProvider(config.AuditSink.Provider)
		if err != nil {
			return fmt.Errorf("failed to load audit sink: %w", err)
		}
		r.auditSink, err = provider.CreateAuditSink(ctx, *config.AuditSink)
		if err != nil {
			return fmt.Errorf("failed to load audit sink: %w", err)
		}
	}

	// Load policy evaluator
	if config.PolicyEvaluator != nil {
		provider, err := r.getProvider(config.PolicyEvaluator.Provider)
		if err != nil {
			return fmt.Errorf("failed to load policy evaluator: %w", err)
		}
		r.policyEvaluator, err = provider.CreatePolicyEvaluator(ctx, *config.PolicyEvaluator)
		if err != nil {
			return fmt.Errorf("failed to load policy evaluator: %w", err)
		}
	}

	// Load MCP provisioner
	if config.MCPProvisioner != nil {
		provider, err := r.getProvider(config.MCPProvisioner.Provider)
		if err != nil {
			return fmt.Errorf("failed to load MCP provisioner: %w", err)
		}
		r.mcpProvisioner, err = provider.CreateMCPProvisioner(ctx, *config.MCPProvisioner)
		if err != nil {
			return fmt.Errorf("failed to load MCP provisioner: %w", err)
		}
	}

	// Load telemetry plugin
	if config.Telemetry != nil {
		provider, err := r.getProvider(config.Telemetry.Provider)
		if err != nil {
			return fmt.Errorf("failed to load telemetry plugin: %w", err)
		}
		r.telemetryPlugin, err = provider.CreateTelemetryPlugin(ctx, *config.Telemetry)
		if err != nil {
			return fmt.Errorf("failed to load telemetry plugin: %w", err)
		}
	}

	return nil
}

// getProvider returns a provider by name, must be called with lock held.
func (r *PluginRegistry) getProvider(name string) (PluginProvider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return provider, nil
}

// AuthProvider returns the loaded auth provider.
func (r *PluginRegistry) AuthProvider() AuthProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.authProvider
}

// CredentialStorage returns the loaded credential storage.
func (r *PluginRegistry) CredentialStorage() CredentialStorage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.credentialStorage
}

// AuthProxy returns the loaded auth proxy.
func (r *PluginRegistry) AuthProxy() AuthProxy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.authProxy
}

// AuditSink returns the loaded audit sink.
func (r *PluginRegistry) AuditSink() AuditSink {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.auditSink
}

// PolicyEvaluator returns the loaded policy evaluator.
func (r *PluginRegistry) PolicyEvaluator() PolicyEvaluator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.policyEvaluator
}

// MCPProvisioner returns the loaded MCP provisioner.
func (r *PluginRegistry) MCPProvisioner() MCPProvisioner {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mcpProvisioner
}

// TelemetryPlugin returns the loaded telemetry plugin.
func (r *PluginRegistry) TelemetryPlugin() TelemetryPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.telemetryPlugin
}

// HasTelemetryPlugin returns true if a telemetry plugin is loaded.
func (r *PluginRegistry) HasTelemetryPlugin() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.telemetryPlugin != nil
}

// RegisterTelemetryPlugin directly registers a telemetry plugin.
// This is used for backwards compatibility with existing telemetry initialization.
func (r *PluginRegistry) RegisterTelemetryPlugin(plugin TelemetryPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.telemetryPlugin != nil {
		return fmt.Errorf("telemetry plugin already registered")
	}
	r.telemetryPlugin = plugin
	return nil
}

// UnregisterTelemetryPlugin removes the registered telemetry plugin.
func (r *PluginRegistry) UnregisterTelemetryPlugin() error {
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

// Close shuts down all loaded plugins.
func (r *PluginRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.telemetryPlugin != nil {
		_ = r.telemetryPlugin.Close()
		r.telemetryPlugin = nil
	}

	// Other plugins don't have Close methods currently
	r.authProvider = nil
	r.credentialStorage = nil
	r.authProxy = nil
	r.auditSink = nil
	r.policyEvaluator = nil
	r.mcpProvisioner = nil

	return nil
}

// ResetForTesting resets the registry state for testing purposes.
func (r *PluginRegistry) ResetForTesting() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.authProvider = nil
	r.credentialStorage = nil
	r.authProxy = nil
	r.auditSink = nil
	r.policyEvaluator = nil
	r.mcpProvisioner = nil
	r.telemetryPlugin = nil
}
