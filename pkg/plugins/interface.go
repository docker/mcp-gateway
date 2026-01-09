package plugins

import (
	"context"
	"io"
)

// PluginClient defines the interface for communicating with plugins.
// This interface abstracts the transport layer, allowing the same plugin
// interface to work with both subprocess (Desktop) and sidecar (Kubernetes)
// deployment models.
type PluginClient interface {
	// Call invokes a method on the plugin with the given parameters.
	// The params are JSON-serializable and the result is returned as raw JSON.
	Call(ctx context.Context, method string, params any) ([]byte, error)

	// Close shuts down the connection to the plugin.
	Close() error
}

// PluginTransport defines how to establish a connection to a plugin.
// Different transports support different deployment models.
type PluginTransport interface {
	// Connect establishes a connection to the plugin and returns a PluginClient.
	Connect(ctx context.Context) (PluginClient, error)
}

// PluginInfo contains metadata about a plugin.
type PluginInfo struct {
	// Name is the unique identifier for the plugin.
	Name string

	// Type indicates the plugin category (e.g., "telemetry", "auth", "audit").
	Type string

	// Version is the plugin version.
	Version string
}

// Plugin represents a loaded plugin instance.
type Plugin interface {
	// Info returns metadata about the plugin.
	Info() PluginInfo

	// Client returns the underlying client for making calls to the plugin.
	Client() PluginClient

	// Close shuts down the plugin and releases resources.
	Close() error
}

// PluginConfig defines the configuration for loading a plugin.
type PluginConfig struct {
	// Name is the plugin name used for registration.
	Name string `json:"name" yaml:"name"`

	// Type is the transport type: "subprocess" or "sidecar".
	Type string `json:"type" yaml:"type"`

	// Subprocess configuration (used when Type is "subprocess").
	Subprocess *SubprocessConfig `json:"subprocess,omitempty" yaml:"subprocess,omitempty"`

	// Sidecar configuration (used when Type is "sidecar").
	Sidecar *SidecarConfig `json:"sidecar,omitempty" yaml:"sidecar,omitempty"`
}

// SubprocessConfig defines configuration for subprocess-based plugins.
type SubprocessConfig struct {
	// Exec is the path to the plugin executable.
	Exec string `json:"exec" yaml:"exec"`

	// Args are command-line arguments to pass to the plugin.
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`

	// Env are environment variables to set for the plugin process.
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`

	// WorkDir is the working directory for the plugin process.
	WorkDir string `json:"workdir,omitempty" yaml:"workdir,omitempty"`
}

// SidecarConfig defines configuration for sidecar-based plugins.
type SidecarConfig struct {
	// URL is the base URL for the plugin's HTTP endpoint.
	URL string `json:"url" yaml:"url"`

	// Headers are additional HTTP headers to include in requests.
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// PluginLifecycleHooks provides callbacks for plugin lifecycle events.
type PluginLifecycleHooks struct {
	// OnStart is called when a plugin starts successfully.
	OnStart func(info PluginInfo)

	// OnStop is called when a plugin stops.
	OnStop func(info PluginInfo, err error)

	// OnError is called when a plugin encounters an error.
	OnError func(info PluginInfo, err error)

	// OnRestart is called when a plugin is restarted.
	OnRestart func(info PluginInfo, attempt int)
}

// PluginOutput represents output streams from a subprocess plugin.
type PluginOutput struct {
	// Stdout is the plugin's standard output stream.
	Stdout io.Reader

	// Stderr is the plugin's standard error stream.
	Stderr io.Reader
}
