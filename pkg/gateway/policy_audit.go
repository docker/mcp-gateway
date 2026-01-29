package gateway

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/policy"
)

type pendingAudit struct {
	client policy.Client
	event  policy.AuditEvent
}

// auditEvents queues pending audits (buffer 100, drops when full).
var auditEvents = make(chan pendingAudit, 100)

func init() {
	go auditWorker()
}

func auditWorker() {
	for pending := range auditEvents {
		// Note: Errors intentionally ignored
		_ = pending.client.SubmitAudit(context.Background(), pending.event)
	}
}

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

// submitAuditEvent queues an audit event, dropping if full.
func submitAuditEvent(client policy.Client, event policy.AuditEvent) {
	if client == nil {
		return
	}
	select {
	case auditEvents <- pendingAudit{client: client, event: event}:
	default:
		log.Log("audit event dropped due to backpressure")
	}
}
