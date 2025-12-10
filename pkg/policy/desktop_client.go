package policy

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// DesktopClient calls the Docker Desktop backend policy endpoint.
type DesktopClient struct {
	client *desktop.RawClient
}

func NewDesktopClient() *DesktopClient {
	return &DesktopClient{
		client: desktop.ClientBackend,
	}
}

func (c *DesktopClient) Evaluate(ctx context.Context, req Request) (Decision, error) {
	var resp Decision
	if err := c.client.Post(ctx, "/mcp/policy/evaluate", req, &resp); err != nil {
		return Decision{Allowed: true}, err
	}
	return resp, nil
}
