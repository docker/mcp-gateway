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
	serverSourceType := resolveServerSourceType(ctx, server)
	target := buildTarget(serverName, tool)

	return policy.Request{
		Catalog:      ctx.Catalog,
		WorkingSet:   ctx.WorkingSet,
		Server:       serverName,
		ServerType:   serverSourceType,
		ServerSource: InferServerSource(serverSourceType, server),
		Transport:    InferServerTransportType(server),
		Tool:         tool,
		Action:       action,
		Target:       target,
	}
}

// resolveServerSourceType determines the policy server source type using the
// request context and server spec.
func resolveServerSourceType(ctx Context, server catalog.Server) string {
	derived := InferServerSourceType(server)
	override := normalizeServerType(ctx.ServerSourceTypeOverride)
	if override == "" {
		return derived
	}
	if derived == "" {
		return override
	}
	if derived == "registry" && override != "registry" {
		return override
	}
	return derived
}

// BuildCatalogRequest creates a policy request for a catalog target.
func BuildCatalogRequest(ctx Context, catalogID string, action policy.Action) policy.Request {
	target := &policy.Target{
		Type: policy.TargetCatalog,
		Name: catalogID,
	}
	return policy.Request{
		Catalog:    catalogID,
		WorkingSet: ctx.WorkingSet,
		Action:     action,
		Target:     target,
	}
}

// BuildWorkingSetRequest creates a policy request for a working set target.
func BuildWorkingSetRequest(ctx Context, workingSetID string, action policy.Action) policy.Request {
	target := &policy.Target{
		Type: policy.TargetWorkingSet,
		Name: workingSetID,
	}
	return policy.Request{
		Catalog:    ctx.Catalog,
		WorkingSet: workingSetID,
		Action:     action,
		Target:     target,
	}
}

// buildTarget derives a policy target from server and tool details.
func buildTarget(serverName, tool string) *policy.Target {
	if tool != "" {
		return &policy.Target{
			Type: policy.TargetTool,
			Name: tool,
		}
	}
	if serverName == "" {
		return nil
	}
	return &policy.Target{
		Type: policy.TargetServer,
		Name: serverName,
	}
}

// InferServerSourceType determines the policy server source type.
func InferServerSourceType(server catalog.Server) string {
	if server.Type != "" {
		if normalized := normalizeServerType(server.Type); normalized != "" {
			return normalized
		}
	}
	if server.Remote.URL != "" || server.SSEEndpoint != "" {
		return "remote"
	}
	if server.Image != "" {
		return "image"
	}
	return ""
}

// normalizeServerType maps legacy catalog types to policy server types.
// Returns empty string when the type is unknown.
func normalizeServerType(serverType string) string {
	switch serverType {
	case "registry", "image", "remote":
		return serverType
	case "server", "poci":
		return "registry"
	default:
		return ""
	}
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
