package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/policy"
)

// auditClientInfo captures client identity details for audit events.
type auditClientInfo struct {
	// Name is the client name.
	Name string
	// Version is the client version.
	Version string
}

// auditClientInfoFromSession extracts client info from an MCP session.
func auditClientInfoFromSession(session *mcp.ServerSession) *auditClientInfo {
	if session == nil {
		return nil
	}

	params := session.InitializeParams()
	if params == nil || params.ClientInfo == nil {
		return nil
	}

	return &auditClientInfo{
		Name:    params.ClientInfo.Name,
		Version: params.ClientInfo.Version,
	}
}

// auditTargetType determines the audit target type for a policy request.
func auditTargetType(req policy.Request) policy.AuditTargetType {
	if req.Action == policy.ActionPrompt {
		return policy.AuditTargetPrompt
	}
	if req.Tool != "" {
		return policy.AuditTargetTool
	}
	return policy.AuditTargetServer
}

// buildAuditEvent builds an audit event for the policy decision.
func buildAuditEvent(
	req policy.Request,
	decision policy.Decision,
	evalErr error,
	clientInfo *auditClientInfo,
) policy.AuditEvent {
	event := policy.AuditEvent{
		Trigger:      req.Action,
		TargetType:   auditTargetType(req),
		ServerName:   req.Server,
		CatalogName:  req.Catalog,
		WorkingSet:   req.WorkingSet,
		ServerType:   req.ServerType,
		ServerSource: req.ServerSource,
		Transport:    req.Transport,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}

	if req.Action == policy.ActionPrompt {
		event.PromptName = req.Tool
	} else if req.Tool != "" {
		event.ToolName = req.Tool
	}

	if clientInfo != nil {
		event.ClientName = clientInfo.Name
		event.ClientVersion = clientInfo.Version
	}

	if evalErr != nil {
		event.Result = policy.AuditResultDenied
		event.OutcomeReason = policy.AuditOutcomePolicyError
		event.Reason = evalErr.Error()
		return event
	}
	if decision.Error != "" {
		event.Result = policy.AuditResultDenied
		event.OutcomeReason = policy.AuditOutcomePolicyError
		event.Reason = decision.Error
		return event
	}
	if decision.Allowed {
		event.Result = policy.AuditResultAllowed
		event.OutcomeReason = policy.AuditOutcomePolicyRule
		return event
	}

	event.Result = policy.AuditResultDenied
	event.OutcomeReason = policy.AuditOutcomePolicyRule
	event.Reason = decision.Reason
	return event
}

// submitAuditEvent submits the audit event asynchronously.
func submitAuditEvent(client policy.Client, event policy.AuditEvent) {
	if client == nil {
		return
	}
	go client.SubmitAudit(context.Background(), event)
}

// normalizePolicyDecisions ensures a decision for each request.
func normalizePolicyDecisions(
	reqs []policy.Request,
	decisions []policy.Decision,
	evalErr error,
) ([]policy.Decision, error) {
	if evalErr == nil && len(decisions) == len(reqs) {
		return decisions, nil
	}

	if evalErr == nil {
		evalErr = fmt.Errorf(
			"batch policy check returned %d decisions for %d requests",
			len(decisions),
			len(reqs),
		)
	}

	normalized := make([]policy.Decision, len(reqs))
	errMsg := evalErr.Error()
	for i := range normalized {
		normalized[i] = policy.Decision{Allowed: false, Error: errMsg}
	}
	return normalized, evalErr
}
