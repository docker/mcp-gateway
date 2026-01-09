package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/plugins"
)

// mcpAuthProvider implements plugins.AuthProvider via MCP tool calls.
type mcpAuthProvider struct {
	provider *Provider
	endpoint string
}

func (a *mcpAuthProvider) ValidateCredential(ctx context.Context, creds plugins.Credentials) (*plugins.UserPrincipal, error) {
	result, err := a.provider.callTool(ctx, a.endpoint, "validate_credential", map[string]any{
		"type":  creds.Type,
		"value": creds.Value,
	})
	if err != nil {
		return nil, err
	}

	var principal plugins.UserPrincipal
	if err := json.Unmarshal(result, &principal); err != nil {
		return nil, fmt.Errorf("failed to parse user principal: %w", err)
	}
	return &principal, nil
}

// mcpCredentialStorage implements plugins.CredentialStorage via MCP tool calls.
type mcpCredentialStorage struct {
	provider *Provider
	endpoint string
}

func (s *mcpCredentialStorage) Store(ctx context.Context, userID, server, credType, value string) error {
	_, err := s.provider.callTool(ctx, s.endpoint, "store_credential", map[string]any{
		"user_id":         userID,
		"mcp_server":      server,
		"credential_type": credType,
		"value":           value,
	})
	return err
}

func (s *mcpCredentialStorage) Retrieve(ctx context.Context, userID, server, credType string) (string, error) {
	result, err := s.provider.callTool(ctx, s.endpoint, "retrieve_credential", map[string]any{
		"user_id":         userID,
		"mcp_server":      server,
		"credential_type": credType,
	})
	if err != nil {
		return "", err
	}

	var resp struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", fmt.Errorf("failed to parse credential: %w", err)
	}
	return resp.Value, nil
}

func (s *mcpCredentialStorage) Delete(ctx context.Context, userID, server, credType string) error {
	_, err := s.provider.callTool(ctx, s.endpoint, "delete_credential", map[string]any{
		"user_id":         userID,
		"mcp_server":      server,
		"credential_type": credType,
	})
	return err
}

func (s *mcpCredentialStorage) List(ctx context.Context, userID string) ([]plugins.CredentialInfo, error) {
	result, err := s.provider.callTool(ctx, s.endpoint, "list_credentials", map[string]any{
		"user_id": userID,
	})
	if err != nil {
		return nil, err
	}

	var infos []plugins.CredentialInfo
	if err := json.Unmarshal(result, &infos); err != nil {
		return nil, fmt.Errorf("failed to parse credential list: %w", err)
	}
	return infos, nil
}

// mcpAuthProxy implements plugins.AuthProxy via MCP tool calls.
type mcpAuthProxy struct {
	provider *Provider
	endpoint string
}

func (a *mcpAuthProxy) InjectCredentials(ctx context.Context, req *plugins.ProxyRequest) (*plugins.ProxyResponse, error) {
	result, err := a.provider.callTool(ctx, a.endpoint, "inject_credentials", map[string]any{
		"user_id":    req.UserID,
		"tenant_id":  req.TenantID,
		"mcp_server": req.MCPServer,
		"target_url": req.TargetURL,
		"method":     req.Method,
		"headers":    req.Headers,
		"body":       req.Body,
	})
	if err != nil {
		return nil, err
	}

	var resp plugins.ProxyResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse proxy response: %w", err)
	}
	return &resp, nil
}

// mcpAuditSink implements plugins.AuditSink via MCP tool calls.
type mcpAuditSink struct {
	provider *Provider
	endpoint string
}

func (s *mcpAuditSink) LogEvent(ctx context.Context, event *plugins.AuditEvent) error {
	_, err := s.provider.callTool(ctx, s.endpoint, "log_event", map[string]any{
		"timestamp":  event.Timestamp,
		"event_type": event.EventType,
		"tenant_id":  event.TenantID,
		"user_id":    event.UserID,
		"mcp_server": event.MCPServer,
		"tool":       event.Tool,
		"result":     event.Result,
		"metadata":   event.Metadata,
	})
	return err
}

// mcpPolicyEvaluator implements plugins.PolicyEvaluator via MCP tool calls.
type mcpPolicyEvaluator struct {
	provider *Provider
	endpoint string
}

func (e *mcpPolicyEvaluator) CheckAccess(ctx context.Context, principal *plugins.UserPrincipal, mcpServer string) error {
	result, err := e.provider.callTool(ctx, e.endpoint, "check_access", map[string]any{
		"user_id":    principal.UserID,
		"tenant_id":  principal.TenantID,
		"roles":      principal.Roles,
		"groups":     principal.Groups,
		"mcp_server": mcpServer,
	})
	if err != nil {
		return err
	}

	var resp struct {
		Allowed bool   `json:"allowed"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return fmt.Errorf("failed to parse access check response: %w", err)
	}

	if !resp.Allowed {
		return fmt.Errorf("access denied: %s", resp.Reason)
	}
	return nil
}

// mcpProvisioner implements plugins.MCPProvisioner via MCP tool calls.
type mcpProvisioner struct {
	provider *Provider
	endpoint string
}

func (p *mcpProvisioner) Provision(ctx context.Context, server *plugins.ServerDef, userID string) (*plugins.ProvisionedServer, error) {
	result, err := p.provider.callTool(ctx, p.endpoint, "provision", map[string]any{
		"server":  server,
		"user_id": userID,
	})
	if err != nil {
		return nil, err
	}

	var provisioned plugins.ProvisionedServer
	if err := json.Unmarshal(result, &provisioned); err != nil {
		return nil, fmt.Errorf("failed to parse provisioned server: %w", err)
	}
	return &provisioned, nil
}

func (p *mcpProvisioner) Deprovision(ctx context.Context, serverID string) error {
	_, err := p.provider.callTool(ctx, p.endpoint, "deprovision", map[string]any{
		"server_id": serverID,
	})
	return err
}

func (p *mcpProvisioner) List(ctx context.Context, userID string) ([]*plugins.ProvisionedServer, error) {
	result, err := p.provider.callTool(ctx, p.endpoint, "list", map[string]any{
		"user_id": userID,
	})
	if err != nil {
		return nil, err
	}

	var servers []*plugins.ProvisionedServer
	if err := json.Unmarshal(result, &servers); err != nil {
		return nil, fmt.Errorf("failed to parse server list: %w", err)
	}
	return servers, nil
}

// mcpTelemetry implements plugins.TelemetryPlugin via MCP tool calls.
type mcpTelemetry struct {
	provider *Provider
	endpoint string
}

func (t *mcpTelemetry) RecordCounter(ctx context.Context, name string, value int64, attrs map[string]string) {
	_, _ = t.provider.callTool(ctx, t.endpoint, "record-counter", map[string]any{
		"name":       name,
		"value":      value,
		"attributes": attrs,
	})
}

func (t *mcpTelemetry) RecordHistogram(ctx context.Context, name string, value float64, attrs map[string]string) {
	_, _ = t.provider.callTool(ctx, t.endpoint, "record-histogram", map[string]any{
		"name":       name,
		"value":      value,
		"attributes": attrs,
	})
}

func (t *mcpTelemetry) RecordGauge(ctx context.Context, name string, value int64, attrs map[string]string) {
	_, _ = t.provider.callTool(ctx, t.endpoint, "record-gauge", map[string]any{
		"name":       name,
		"value":      value,
		"attributes": attrs,
	})
}

func (t *mcpTelemetry) Close() error {
	return nil
}
