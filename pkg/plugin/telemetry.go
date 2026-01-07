// Package plugin provides interfaces and registry for MCP Gateway plugins.
package plugin

import (
	"context"
)

// TelemetryPlugin defines the interface for telemetry plugins.
// This interface mirrors OpenTelemetry's metric instruments, allowing
// plugins to receive telemetry data in a generic format that can be
// processed by any backend.
type TelemetryPlugin interface {
	// RecordCounter increments a counter metric.
	// Counters are cumulative metrics that only increase (e.g., number of requests).
	RecordCounter(ctx context.Context, name string, value int64, attrs map[string]string)

	// RecordHistogram records a value in a histogram metric.
	// Histograms track the distribution of values (e.g., request durations).
	RecordHistogram(ctx context.Context, name string, value float64, attrs map[string]string)

	// RecordGauge records a point-in-time value.
	// Gauges represent values that can go up or down (e.g., current connections).
	RecordGauge(ctx context.Context, name string, value int64, attrs map[string]string)

	// Close shuts down the plugin and releases any resources.
	Close() error
}
