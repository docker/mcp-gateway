package oauth

import "context"

// Logger is an interface for logging during OAuth discovery
// Implementations should log with appropriate formatting and destination
type Logger interface {
	Infof(format string, args ...any)  // Informational messages
	Warnf(format string, args ...any)  // Warnings (non-fatal issues)
	Debugf(format string, args ...any) // Debug/verbose details
}

type contextKey struct{}

var loggerKey = contextKey{}

// WithLogger attaches a logger to the context
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// loggerFromContext extracts the logger from context
// Returns a noop logger if none is set (for backward compatibility)
func loggerFromContext(ctx context.Context) Logger {
	if logger, ok := ctx.Value(loggerKey).(Logger); ok {
		return logger
	}
	return noopLogger{}
}

// noopLogger does nothing (used when no logger is provided)
type noopLogger struct{}

func (noopLogger) Infof(_ string, _ ...any)  {}
func (noopLogger) Warnf(_ string, _ ...any)  {}
func (noopLogger) Debugf(_ string, _ ...any) {}
