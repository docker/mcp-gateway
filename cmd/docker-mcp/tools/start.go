package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	client "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

func start(ctx context.Context, version string, gatewayArgs []string, debug bool) (client.Client, error) {
	var args []string
	if version == "2" {
		args = []string{"mcp", "gateway", "run"}
	} else {
		args = []string{"run", "-i", "--rm", "alpine/socat", "STDIO", "TCP:host.docker.internal:8811"}
	}
	args = append(args, gatewayArgs...)

	c := client.NewStdioCmdClient("gateway", "docker", nil, args...)
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "docker",
		Version: "1.0.0",
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if _, err := c.Initialize(ctx, initRequest, debug); err != nil {
		return nil, fmt.Errorf("initializing: %w", err)
	}

	return c, nil
}
