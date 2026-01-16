package policy

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// Action identifies the type of operation being evaluated.
// It is optional; when empty callers should treat it as "invoke".
type Action string

const (
	ActionLoad   Action = "load"   // loading/listing configuration/catalog
	ActionInvoke Action = "invoke" // tool invocation (default)
	ActionPrompt Action = "prompt" // prompt retrieval
)

// TargetType identifies the policy target type.
type TargetType string

const (
	// TargetCatalog identifies a catalog target.
	TargetCatalog TargetType = "catalog"
	// TargetWorkingSet identifies a working set target.
	TargetWorkingSet TargetType = "workingSet"
	// TargetServer identifies a server target.
	TargetServer TargetType = "server"
	// TargetTool identifies a tool target.
	TargetTool TargetType = "tool"
)

// Target identifies the policy target for a request.
type Target struct {
	// Type identifies the target type.
	Type TargetType `json:"type,omitempty"`
	// Name identifies the target name.
	Name string `json:"name,omitempty"`
}

// Request is a policy evaluation request.
type Request struct {
	Catalog string `json:"catalog,omitempty"`
	// WorkingSet identifies the working set (profile) for the request.
	WorkingSet string `json:"workingSet,omitempty"`
	Server     string `json:"server,omitempty"`
	// ServerType identifies the server source type for the request.
	ServerType string `json:"serverType,omitempty"`
	// ServerSource identifies the server source for the request.
	ServerSource string `json:"serverSource,omitempty"`
	// Transport identifies the server transport type for the request.
	Transport string `json:"transport,omitempty"`
	// Tool identifies the tool name for the request.
	Tool string `json:"tool,omitempty"`
	// Action identifies the action for the request.
	Action Action `json:"action,omitempty"`
	// Target identifies the policy target for the request.
	Target *Target `json:"target,omitempty"`
}

// Decision is a policy evaluation result.
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
	// Error is the error string for evaluation failures.
	Error string `json:"error,omitempty"`
}

// Client performs policy checks.
type Client interface {
	// Evaluate performs a single policy evaluation.
	Evaluate(ctx context.Context, req Request) (Decision, error)
	// EvaluateBatch performs multiple policy evaluations in a single call.
	// Returns decisions in the same order as requests.
	EvaluateBatch(ctx context.Context, reqs []Request) ([]Decision, error)
	// SubmitAudit submits a policy audit event.
	SubmitAudit(ctx context.Context, event AuditEvent) error
}

// NoopClient always allows.
type NoopClient struct{}

// Evaluate always returns an allowed decision.
func (NoopClient) Evaluate(_ context.Context, _ Request) (Decision, error) {
	return Decision{Allowed: true}, nil
}

// EvaluateBatch returns allowed decisions for all requests.
func (NoopClient) EvaluateBatch(_ context.Context, reqs []Request) ([]Decision, error) {
	decisions := make([]Decision, len(reqs))
	for i := range decisions {
		decisions[i] = Decision{Allowed: true}
	}
	return decisions, nil
}

// SubmitAudit ignores audit events for the noop client.
func (NoopClient) SubmitAudit(_ context.Context, _ AuditEvent) error {
	return nil
}

// NewDefaultClient returns a policy client appropriate for the current context.
func NewDefaultClient(ctx context.Context) Client {
	if !desktop.IsRunningInDockerDesktop(ctx) {
		return NoopClient{}
	}
	return NewDesktopClient()
}
