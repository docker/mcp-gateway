package policy

// AuditTargetType identifies the resource kind for an audit event.
type AuditTargetType string

const (
	// AuditTargetServer identifies a server target.
	AuditTargetServer AuditTargetType = "server"
	// AuditTargetTool identifies a tool target.
	AuditTargetTool AuditTargetType = "tool"
	// AuditTargetPrompt identifies a prompt target.
	AuditTargetPrompt AuditTargetType = "prompt"
)

// AuditResult identifies the policy decision outcome.
type AuditResult string

const (
	// AuditResultAllowed indicates the policy allowed the action.
	AuditResultAllowed AuditResult = "allowed"
	// AuditResultDenied indicates the policy denied the action.
	AuditResultDenied AuditResult = "denied"
)

// AuditOutcomeReason identifies why the result occurred.
type AuditOutcomeReason string

const (
	// AuditOutcomePolicyRule indicates a policy rule determined the outcome.
	AuditOutcomePolicyRule AuditOutcomeReason = "policy_rule"
	// AuditOutcomePolicyError indicates evaluation failed due to an error.
	AuditOutcomePolicyError AuditOutcomeReason = "policy_error"
)

// AuditEvent represents a policy evaluation audit event.
type AuditEvent struct {
	// ActorType identifies the actor type.
	ActorType string `json:"actor_type,omitempty"`
	// ActorID identifies the actor identifier.
	ActorID string `json:"actor_id,omitempty"`
	// ActorName identifies the actor display name.
	ActorName string `json:"actor_name,omitempty"`
	// OrgID identifies the organization identifier.
	OrgID string `json:"org_id,omitempty"`
	// OrgName identifies the organization name.
	OrgName string `json:"org_name,omitempty"`
	// Trigger identifies the operation that triggered evaluation.
	Trigger Action `json:"trigger"`
	// TargetType identifies the evaluated resource type.
	TargetType AuditTargetType `json:"target_type"`
	// ServerName identifies the server name.
	ServerName string `json:"server_name"`
	// ToolName identifies the tool name, when applicable.
	ToolName string `json:"tool_name,omitempty"`
	// PromptName identifies the prompt name, when applicable.
	PromptName string `json:"prompt_name,omitempty"`
	// CatalogName identifies the catalog identifier, when available.
	CatalogName string `json:"catalog_name,omitempty"`
	// WorkingSet identifies the working set identifier, when available.
	WorkingSet string `json:"working_set,omitempty"`
	// ServerType identifies the server source type, when available.
	ServerType string `json:"server_type,omitempty"`
	// ServerSource identifies the server source identifier, when available.
	ServerSource string `json:"server_source,omitempty"`
	// Transport identifies the server transport type, when available.
	Transport string `json:"transport,omitempty"`
	// Result identifies the policy decision outcome.
	Result AuditResult `json:"result"`
	// OutcomeReason identifies why the outcome occurred.
	OutcomeReason AuditOutcomeReason `json:"outcome_reason,omitempty"`
	// Reason provides a human-readable explanation for the outcome.
	Reason string `json:"reason,omitempty"`
	// PolicyID identifies the policy identifier, when available.
	PolicyID string `json:"policy_id,omitempty"`
	// PolicyVersion identifies the policy version, when available.
	PolicyVersion string `json:"policy_version,omitempty"`
	// PolicySource identifies the policy source, when available.
	PolicySource string `json:"policy_source,omitempty"`
	// ClientName identifies the client name, when available.
	ClientName string `json:"client_name,omitempty"`
	// ClientVersion identifies the client version, when available.
	ClientVersion string `json:"client_version,omitempty"`
	// SessionID identifies the client session identifier, when available.
	SessionID string `json:"session_id,omitempty"`
	// TraceID identifies the trace identifier, when available.
	TraceID string `json:"trace_id,omitempty"`
	// Timestamp identifies when the evaluation occurred.
	Timestamp string `json:"timestamp"`
}

// AuditResponse represents the audit submission response.
type AuditResponse struct {
	// Accepted indicates whether the event was accepted.
	Accepted bool `json:"accepted"`
	// Message provides additional response detail, when available.
	Message string `json:"message,omitempty"`
}
