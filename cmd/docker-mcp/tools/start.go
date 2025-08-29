package tools

import (
	"context"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func start(ctx context.Context, version string, gatewayArgs []string, _ bool) (*mcp.ClientSession, error) {
	var args []string
	if version == "2" {
		args = []string{"mcp", "gateway", "run"}
	} else {
		args = []string{"run", "-i", "--rm", "alpine/socat", "STDIO", "TCP:host.docker.internal:8811"}
	}
	args = append(args, gatewayArgs...)

	c := mcp.NewClient(&mcp.Implementation{Name: "mcp-gateway-client", Version: "1.0.0"}, nil)
	transport := &mcp.CommandTransport{Command: exec.CommandContext(ctx, "docker", args...)}
	session, err := c.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}

	return session, nil
}
