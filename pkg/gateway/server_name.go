package gateway

import (
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultGatewayServerName = "Docker AI MCP Gateway"
	gatewayServerVersion     = "2.0.1"
)

func (g *Gateway) serverImplementation() *mcp.Implementation {
	name := defaultGatewayServerName
	if trimmed := strings.TrimSpace(g.GatewayName); trimmed != "" {
		name = trimmed
	}
	return &mcp.Implementation{
		Name:    name,
		Version: gatewayServerVersion,
	}
}
