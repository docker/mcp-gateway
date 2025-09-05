package tools

import (
	"context"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/logs"
)

func start(ctx context.Context, version string, gatewayArgs []string, verbose bool) (*mcp.ClientSession, error) {
	var args []string
	if version == "2" {
		if verbose {
			args = []string{"mcp", "gateway", "run", "--verbose"}
		} else {
			args = []string{"mcp", "gateway", "run"}
		}
	} else {
		args = []string{"run", "-i", "--rm", "alpine/socat", "STDIO", "TCP:host.docker.internal:8811"}
	}
	args = append(args, gatewayArgs...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if verbose {
		cmd.Stderr = logs.NewPrefixer(os.Stderr, "- mcp-gateway: ")
	}

	c := mcp.NewClient(&mcp.Implementation{Name: "mcp-gateway-client", Version: "1.0.0"}, nil)
	transport := &mcp.CommandTransport{Command: cmd}
	session, err := c.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}

	return session, nil
}
