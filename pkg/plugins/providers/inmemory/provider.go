// Package inmemory provides in-memory plugin implementations.
//
// In-memory plugins run Go code directly in the gateway process.
// They provide the lowest latency but no isolation from the gateway.
//
// Use cases:
//   - Simple plugins (always-allow policy, stdout logger)
//   - Performance-critical hot paths (authentication validation)
//   - Plugins without external dependencies
package inmemory

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/mcp-gateway/pkg/plugins"
)

// Provider implements plugins.PluginProvider for in-memory plugins.
type Provider struct {
	mu              sync.RWMutex
	authProviders   map[string]plugins.AuthProvider
	credStorages    map[string]plugins.CredentialStorage
	authProxies     map[string]plugins.AuthProxy
	auditSinks      map[string]plugins.AuditSink
	policyEvals     map[string]plugins.PolicyEvaluator
	mcpProvisioners map[string]plugins.MCPProvisioner
	telemetry       map[string]plugins.TelemetryPlugin
}

// NewProvider creates a new in-memory provider.
func NewProvider() *Provider {
	return &Provider{
		authProviders:   make(map[string]plugins.AuthProvider),
		credStorages:    make(map[string]plugins.CredentialStorage),
		authProxies:     make(map[string]plugins.AuthProxy),
		auditSinks:      make(map[string]plugins.AuditSink),
		policyEvals:     make(map[string]plugins.PolicyEvaluator),
		mcpProvisioners: make(map[string]plugins.MCPProvisioner),
		telemetry:       make(map[string]plugins.TelemetryPlugin),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "in-memory"
}

// RegisterAuthProvider registers an in-memory AuthProvider implementation.
func (p *Provider) RegisterAuthProvider(name string, impl plugins.AuthProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.authProviders[name] = impl
}

// RegisterCredentialStorage registers an in-memory CredentialStorage implementation.
func (p *Provider) RegisterCredentialStorage(name string, impl plugins.CredentialStorage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.credStorages[name] = impl
}

// RegisterAuthProxy registers an in-memory AuthProxy implementation.
func (p *Provider) RegisterAuthProxy(name string, impl plugins.AuthProxy) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.authProxies[name] = impl
}

// RegisterAuditSink registers an in-memory AuditSink implementation.
func (p *Provider) RegisterAuditSink(name string, impl plugins.AuditSink) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.auditSinks[name] = impl
}

// RegisterPolicyEvaluator registers an in-memory PolicyEvaluator implementation.
func (p *Provider) RegisterPolicyEvaluator(name string, impl plugins.PolicyEvaluator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.policyEvals[name] = impl
}

// RegisterMCPProvisioner registers an in-memory MCPProvisioner implementation.
func (p *Provider) RegisterMCPProvisioner(name string, impl plugins.MCPProvisioner) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mcpProvisioners[name] = impl
}

// RegisterTelemetryPlugin registers an in-memory TelemetryPlugin implementation.
func (p *Provider) RegisterTelemetryPlugin(name string, impl plugins.TelemetryPlugin) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.telemetry[name] = impl
}

// CreateAuthProvider creates an AuthProvider from configuration.
func (p *Provider) CreateAuthProvider(_ context.Context, config plugins.PluginConfig) (plugins.AuthProvider, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	impl, ok := p.authProviders[config.Implementation]
	if !ok {
		return nil, fmt.Errorf("unknown in-memory auth provider implementation: %s", config.Implementation)
	}
	return impl, nil
}

// CreateCredentialStorage creates a CredentialStorage from configuration.
func (p *Provider) CreateCredentialStorage(_ context.Context, config plugins.PluginConfig) (plugins.CredentialStorage, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	impl, ok := p.credStorages[config.Implementation]
	if !ok {
		return nil, fmt.Errorf("unknown in-memory credential storage implementation: %s", config.Implementation)
	}
	return impl, nil
}

// CreateAuthProxy creates an AuthProxy from configuration.
func (p *Provider) CreateAuthProxy(_ context.Context, config plugins.PluginConfig) (plugins.AuthProxy, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	impl, ok := p.authProxies[config.Implementation]
	if !ok {
		return nil, fmt.Errorf("unknown in-memory auth proxy implementation: %s", config.Implementation)
	}
	return impl, nil
}

// CreateAuditSink creates an AuditSink from configuration.
func (p *Provider) CreateAuditSink(_ context.Context, config plugins.PluginConfig) (plugins.AuditSink, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	impl, ok := p.auditSinks[config.Implementation]
	if !ok {
		return nil, fmt.Errorf("unknown in-memory audit sink implementation: %s", config.Implementation)
	}
	return impl, nil
}

// CreatePolicyEvaluator creates a PolicyEvaluator from configuration.
func (p *Provider) CreatePolicyEvaluator(_ context.Context, config plugins.PluginConfig) (plugins.PolicyEvaluator, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	impl, ok := p.policyEvals[config.Implementation]
	if !ok {
		return nil, fmt.Errorf("unknown in-memory policy evaluator implementation: %s", config.Implementation)
	}
	return impl, nil
}

// CreateMCPProvisioner creates an MCPProvisioner from configuration.
func (p *Provider) CreateMCPProvisioner(_ context.Context, config plugins.PluginConfig) (plugins.MCPProvisioner, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	impl, ok := p.mcpProvisioners[config.Implementation]
	if !ok {
		return nil, fmt.Errorf("unknown in-memory MCP provisioner implementation: %s", config.Implementation)
	}
	return impl, nil
}

// CreateTelemetryPlugin creates a TelemetryPlugin from configuration.
func (p *Provider) CreateTelemetryPlugin(_ context.Context, config plugins.PluginConfig) (plugins.TelemetryPlugin, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	impl, ok := p.telemetry[config.Implementation]
	if !ok {
		return nil, fmt.Errorf("unknown in-memory telemetry plugin implementation: %s", config.Implementation)
	}
	return impl, nil
}

// Verify Provider implements plugins.PluginProvider
var _ plugins.PluginProvider = (*Provider)(nil)
