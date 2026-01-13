package policy

import "context"

// Action identifies the type of operation being evaluated.
// It is optional; when empty callers should treat it as "invoke".
type Action string

const (
	ActionLoad   Action = "load"   // loading/listing configuration/catalog
	ActionInvoke Action = "invoke" // tool invocation (default)
	ActionPrompt Action = "prompt" // prompt retrieval
)

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
	Tool      string `json:"tool,omitempty"`
	Action    Action `json:"action,omitempty"`
}

// Decision is a policy evaluation result.
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// Client performs policy checks.
type Client interface {
	Evaluate(ctx context.Context, req Request) (Decision, error)
}

// NoopClient always allows.
type NoopClient struct{}

func (NoopClient) Evaluate(_ context.Context, _ Request) (Decision, error) {
	return Decision{Allowed: true}, nil
}
