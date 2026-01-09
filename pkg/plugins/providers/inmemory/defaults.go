package inmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/docker/mcp-gateway/pkg/plugins"
)

// DesktopImplicitAuth is an AuthProvider that always returns a desktop-user principal.
// This is used for Desktop deployments where there is implicit trust.
type DesktopImplicitAuth struct{}

// ValidateCredential always returns a desktop-user principal.
func (d *DesktopImplicitAuth) ValidateCredential(_ context.Context, _ plugins.Credentials) (*plugins.UserPrincipal, error) {
	return &plugins.UserPrincipal{
		UserID: "desktop-user",
		Roles:  []string{"admin"},
	}, nil
}

// AlwaysAllowPolicy is a PolicyEvaluator that always allows access.
type AlwaysAllowPolicy struct{}

// CheckAccess always returns nil (allowed).
func (a *AlwaysAllowPolicy) CheckAccess(_ context.Context, _ *plugins.UserPrincipal, _ string) error {
	return nil
}

// StdoutAuditSink is an AuditSink that writes JSON events to stdout.
type StdoutAuditSink struct{}

// LogEvent writes the audit event as JSON to stdout.
func (s *StdoutAuditSink) LogEvent(_ context.Context, event *plugins.AuditEvent) error {
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(data))
	return nil
}

// StderrAuditSink is an AuditSink that writes JSON events to stderr.
type StderrAuditSink struct{}

// LogEvent writes the audit event as JSON to stderr.
func (s *StderrAuditSink) LogEvent(_ context.Context, event *plugins.AuditEvent) error {
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}
	fmt.Fprintln(os.Stderr, string(data))
	return nil
}

// NoopTelemetry is a TelemetryPlugin that discards all metrics.
type NoopTelemetry struct{}

// RecordCounter does nothing.
func (n *NoopTelemetry) RecordCounter(_ context.Context, _ string, _ int64, _ map[string]string) {}

// RecordHistogram does nothing.
func (n *NoopTelemetry) RecordHistogram(_ context.Context, _ string, _ float64, _ map[string]string) {
}

// RecordGauge does nothing.
func (n *NoopTelemetry) RecordGauge(_ context.Context, _ string, _ int64, _ map[string]string) {}

// Close does nothing.
func (n *NoopTelemetry) Close() error { return nil }

// RegisterDefaults registers all default in-memory implementations.
func RegisterDefaults(p *Provider) {
	// Auth providers
	p.RegisterAuthProvider("desktop-implicit", &DesktopImplicitAuth{})

	// Policy evaluators
	p.RegisterPolicyEvaluator("always-allow", &AlwaysAllowPolicy{})

	// Audit sinks
	p.RegisterAuditSink("stdout", &StdoutAuditSink{})
	p.RegisterAuditSink("stderr", &StderrAuditSink{})

	// Telemetry
	p.RegisterTelemetryPlugin("noop", &NoopTelemetry{})
}
