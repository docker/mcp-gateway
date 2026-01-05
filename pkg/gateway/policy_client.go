package gateway

import (
	"os"

	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/policy"
)

func newPolicyClient() policy.Client {
	paths := desktop.Paths()
	if _, err := os.Stat(paths.BackendSocket); err != nil {
		return policy.NoopClient{}
	}
	return policy.NewDesktopClient()
}
