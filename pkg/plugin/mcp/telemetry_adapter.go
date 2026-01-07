// Package mcp provides MCP-based plugin implementations.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/plugin"
)

// TelemetryAdapter implements plugin.TelemetryPlugin using MCP tool calls.
// It translates generic telemetry interface calls (counters, histograms, gauges)
// into MCP tool calls to a telemetry server.
type TelemetryAdapter struct {
	session  *mcp.ClientSession
	client   *mcp.Client
	mu       sync.RWMutex
	endpoint string
}

// NewTelemetryAdapter creates a new MCP telemetry adapter.
// It connects to the telemetry server at the specified host and port.
func NewTelemetryAdapter(ctx context.Context, host string, port int) (*TelemetryAdapter, error) {
	endpoint := fmt.Sprintf("http://%s:%d/mcp", host, port)

	debugLog("Connecting to telemetry server at %s", endpoint)

	transport := &mcp.StreamableClientTransport{
		Endpoint: endpoint,
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "mcp-gateway-telemetry-client",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to telemetry server: %w", err)
	}

	debugLog("Connected to telemetry server")

	return &TelemetryAdapter{
		session:  session,
		client:   client,
		endpoint: endpoint,
	}, nil
}

// RecordCounter implements plugin.TelemetryPlugin.
func (a *TelemetryAdapter) RecordCounter(ctx context.Context, name string, value int64, attrs map[string]string) {
	_ = a.callTool(ctx, "record-counter", map[string]any{
		"name":       name,
		"value":      value,
		"attributes": attrs,
	})
}

// RecordHistogram implements plugin.TelemetryPlugin.
func (a *TelemetryAdapter) RecordHistogram(ctx context.Context, name string, value float64, attrs map[string]string) {
	_ = a.callTool(ctx, "record-histogram", map[string]any{
		"name":       name,
		"value":      value,
		"attributes": attrs,
	})
}

// RecordGauge implements plugin.TelemetryPlugin.
func (a *TelemetryAdapter) RecordGauge(ctx context.Context, name string, value int64, attrs map[string]string) {
	_ = a.callTool(ctx, "record-gauge", map[string]any{
		"name":       name,
		"value":      value,
		"attributes": attrs,
	})
}

// Close implements plugin.TelemetryPlugin.
func (a *TelemetryAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session != nil {
		err := a.session.Close()
		a.session = nil
		return err
	}
	return nil
}

// callTool makes an MCP tool call to the telemetry server.
func (a *TelemetryAdapter) callTool(ctx context.Context, toolName string, args any) error {
	a.mu.RLock()
	session := a.session
	a.mu.RUnlock()

	if session == nil {
		debugLog("Session not initialized, skipping %s", toolName)
		return nil
	}

	argsMap, ok := args.(map[string]any)
	if !ok {
		// Convert struct to map
		data, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("failed to marshal args: %w", err)
		}
		if err := json.Unmarshal(data, &argsMap); err != nil {
			return fmt.Errorf("failed to unmarshal args to map: %w", err)
		}
	}

	debugLog("Calling tool %s", toolName)

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: argsMap,
	})

	if err != nil {
		debugLog("Error calling tool %s: %v", toolName, err)
	} else {
		debugLog("Tool %s returned: %d content items", toolName, len(result.Content))
	}

	return err
}

// Verify TelemetryAdapter implements plugin.TelemetryPlugin
var _ plugin.TelemetryPlugin = (*TelemetryAdapter)(nil)

func debugLog(format string, args ...any) {
	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY-ADAPTER] "+format+"\n", args...)
	}
}
