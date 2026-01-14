package policycontext

import (
	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/policy"
)

// Context carries policy request context values independent of server details.
type Context struct {
	// Catalog is the catalog identifier for the server.
	Catalog string
	// WorkingSet is the working set (profile) identifier for the request.
	WorkingSet string
	// ServerSourceTypeOverride forces the server source type if non-empty.
	ServerSourceTypeOverride string
}

// BuildRequest creates a policy request from context, server, and action data.
func BuildRequest(
	ctx Context,
	serverName string,
	server catalog.Server,
	tool string,
	action policy.Action,
) policy.Request {
	serverSourceType := ctx.ServerSourceTypeOverride
	if serverSourceType == "" {
		serverSourceType = InferServerSourceType(server)
	}

	return policy.Request{
		Catalog:      ctx.Catalog,
		WorkingSet:   ctx.WorkingSet,
		Server:       serverName,
		ServerType:   serverSourceType,
		ServerSource: InferServerSource(serverSourceType, server),
		Transport:    InferServerTransportType(server),
		Tool:         tool,
		Action:       action,
	}
}

// InferServerSourceType determines the policy server source type.
func InferServerSourceType(server catalog.Server) string {
	if server.Type != "" {
		return server.Type
	}
	if server.Remote.URL != "" || server.SSEEndpoint != "" {
		return "remote"
	}
	if server.Image != "" {
		return "image"
	}
	return ""
}

// InferServerSource determines the policy server source identifier.
func InferServerSource(serverSourceType string, server catalog.Server) string {
	if serverSourceType == "registry" && server.Image != "" {
		return server.Image
	}
	if serverSourceType == "image" && server.Image != "" {
		return server.Image
	}
	if serverSourceType == "remote" {
		return InferServerEndpoint(server)
	}
	if server.Image != "" {
		return server.Image
	}
	return InferServerEndpoint(server)
}

// InferServerEndpoint returns the best endpoint for a remote server.
func InferServerEndpoint(server catalog.Server) string {
	if server.SSEEndpoint != "" {
		return server.SSEEndpoint
	}
	if server.Remote.URL != "" {
		return server.Remote.URL
	}
	return ""
}

// InferServerTransportType determines the policy transport type.
// The policy vocabulary uses stdio, sse, and streamable.
func InferServerTransportType(server catalog.Server) string {
	if server.SSEEndpoint != "" {
		return "sse"
	}
	switch server.Remote.Transport {
	case "sse":
		return "sse"
	case "http":
		return "streamable"
	}
	if server.Remote.URL != "" {
		return "streamable"
	}
	if server.Image != "" {
		return "stdio"
	}
	return ""
}
