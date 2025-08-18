package serverless

import (
	"context"
	"time"
)
type DeployedResource struct {
	ConfigPath string
	Namespace  string
	ServerName string
}

// Client defines the interface for managing serverless deployments
type Client interface {
	// DeployMCPServer deploys an MCP server using the specified configuration
	DeployMCPServer(ctx context.Context, serverName, configPath, namespace string) (string, error)

	// DeleteMCPServer deletes a deployed MCP server by name
	DeleteMCPServer(ctx context.Context, serverName string) error

	// WaitForComposition waits for a composition to reach running state within the timeout
	WaitForComposition(ctx context.Context, compositionName, namespace string, timeout time.Duration) error

	// GetServiceEndpoint retrieves the external endpoint for a service
	GetServiceEndpoint(ctx context.Context, serviceName, namespace string) (string, error)

	// CleanupAll removes all deployed resources
	CleanupAll(ctx context.Context) error
}
