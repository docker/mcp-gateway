package policy

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// DesktopClient calls the Docker Desktop backend policy endpoint.
type DesktopClient struct {
	client *desktop.RawClient
}

// NewDesktopClient creates a new Desktop policy client.
func NewDesktopClient() *DesktopClient {
	return &DesktopClient{
		client: desktop.ClientBackend,
	}
}

// Evaluate performs a single policy evaluation via the Desktop backend.
func (c *DesktopClient) Evaluate(ctx context.Context, req Request) (Decision, error) {
	var resp Decision
	if err := c.client.Post(ctx, "/mcp/policy/evaluate", req, &resp); err != nil {
		return Decision{Allowed: false, Error: err.Error()}, err
	}
	return resp, nil
}

// batchRequest is the request body for batch policy evaluation.
type batchRequest struct {
	Requests []Request `json:"requests"`
}

// batchResponse is the response body from batch policy evaluation.
type batchResponse struct {
	Decisions []Decision `json:"decisions"`
}

// EvaluateBatch performs multiple policy evaluations in a single HTTP request.
func (c *DesktopClient) EvaluateBatch(
	ctx context.Context,
	reqs []Request,
) ([]Decision, error) {
	var resp batchResponse
	err := c.client.Post(ctx, "/mcp/policy/evaluate/batch", batchRequest{Requests: reqs}, &resp)
	if err != nil {
		decisions := make([]Decision, len(reqs))
		for i := range decisions {
			decisions[i] = Decision{Allowed: false, Error: err.Error()}
		}
		return decisions, err
	}
	return resp.Decisions, nil
}

// SubmitAudit submits an audit event via the Desktop backend.
func (c *DesktopClient) SubmitAudit(ctx context.Context, event AuditEvent) error {
	var resp AuditResponse
	if err := c.client.Post(ctx, "/mcp/audit", event, &resp); err != nil {
		return err
	}
	return nil
}
