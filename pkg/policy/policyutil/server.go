package policyutil

import "github.com/docker/mcp-gateway/pkg/catalog"

// AllowedToolCount returns the number of tools not blocked by policy.
func AllowedToolCount(tools []catalog.Tool) int {
	count := 0
	for _, tool := range tools {
		if tool.Policy == nil || tool.Policy.Allowed {
			count++
		}
	}
	return count
}

// ServerSpecFromSnapshot returns a policy server spec and name for evaluation.
func ServerSpecFromSnapshot(
	snapshot *catalog.Server,
	serverType string,
	source string,
	image string,
	endpoint string,
) (catalog.Server, string) {
	if snapshot != nil {
		name := snapshot.Name
		if name == "" {
			name = FallbackServerName(serverType, source, image, endpoint)
		}
		return *snapshot, name
	}

	spec := catalog.Server{
		Type:  serverType,
		Image: image,
	}
	switch serverType {
	case "registry":
		if source != "" {
			spec.Image = source
		}
	case "remote":
		spec.Remote = catalog.Remote{URL: endpoint}
	}

	return spec, FallbackServerName(serverType, source, image, endpoint)
}

// FallbackServerName returns a best-effort name when a snapshot is missing.
func FallbackServerName(serverType, source, image, endpoint string) string {
	switch serverType {
	case "registry":
		return source
	case "image":
		return image
	case "remote":
		return endpoint
	}
	return ""
}
