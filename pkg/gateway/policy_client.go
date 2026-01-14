package gateway

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/policy"
)

func newPolicyClient(ctx context.Context) policy.Client {
	return policy.NewDefaultClient(ctx)
}
