package gateway

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/policy"
)

func newPolicyClient(ctx context.Context) policy.Client {
	if !desktop.IsRunningInDockerDesktop(ctx) {
		return policy.NoopClient{}
	}
	return policy.NewDesktopClient()
}
