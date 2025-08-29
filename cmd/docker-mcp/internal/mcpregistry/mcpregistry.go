package mcpregistry

import (
	"encoding/json"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

// Registry represents a JSON registry response with server definitions
type Registry struct {
	Servers map[string]catalog.Server `json:"servers"`
}

// Parse parses JSON content from a registry URL and returns a map of server definitions
func Parse(jsonData []byte) (map[string]catalog.Server, error) {
	var registry Registry
	if err := json.Unmarshal(jsonData, &registry); err != nil {
		return nil, err
	}

	return registry.Servers, nil
}
