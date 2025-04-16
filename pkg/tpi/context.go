package tpi

import (
	"context"
	"log"
	"time"
	// Import for BMCSSHConfig
)

// --- tpi execution context implementation ---

// tpiContext implements the tpi.Context interface.
type tpiContext struct {
	context.Context        // Embed standard context for cancellation
	log             Logger // Logger
}

// Deadline returns the time when work done on behalf of this context
// should be canceled.
func (c *tpiContext) Deadline() (deadline time.Time, ok bool) {
	return c.Context.Deadline()
}

// Done returns a channel that's closed when work done on behalf of this
// context should be canceled.
func (c *tpiContext) Done() <-chan struct{} {
	return c.Context.Done()
}

// Err returns nil if Done is not yet closed. If Done is closed, Err returns a
// non-nil error explaining why: Canceled if the context was canceled
// or DeadlineExceeded if the context's deadline passed.
func (c *tpiContext) Err() error {
	return c.Context.Err()
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Successive calls to Value with
// the same key returns the same result.
func (c *tpiContext) Value(key any) any {
	if key == loggerContextKey {
		return c.log
	}
	return c.Context.Value(key)
}

// Context keys for value lookup
type contextKey string

const (
	loggerContextKey contextKey = "tpiLogger"
)

// Log returns the context's logger.
func (c *tpiContext) Log() Logger {
	return c.log
}

// NewContext creates a new tpi execution context using the provided Go context.
func NewContext(ctx context.Context) Context {
	return &tpiContext{
		Context: ctx,
		log:     DefaultLogger(), // Use default logger for now
	}
}

// NewContextWithLogger creates a new tpi execution context with a custom logger.
func NewContextWithLogger(ctx context.Context, logger Logger) Context {
	return &tpiContext{
		Context: ctx,
		log:     logger,
	}
}

// Logger defines the interface for logging within the TPI system.
type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
	Fatalf(format string, v ...interface{})
}

// DefaultLogger returns a simple logger implementation.
// In a more complete implementation, this could be enhanced with log levels,
// formatting options, etc.
func DefaultLogger() Logger {
	return &defaultLogger{}
}

// defaultLogger is a basic implementation of the Logger interface.
type defaultLogger struct{}

func (l *defaultLogger) Printf(format string, v ...interface{}) {
	// Using the standard Go log package for now
	log.Printf(format, v...)
}

func (l *defaultLogger) Println(v ...interface{}) {
	log.Println(v...)
}

func (l *defaultLogger) Fatalf(format string, v ...interface{}) {
	log.Fatalf(format, v...)
}
