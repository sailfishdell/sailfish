package log

import (
	"context"

	eh "github.com/looplab/eventhorizon"
)

type requestIDType int

const (
	requestIDKey requestIDType = iota
	sessionIDKey
)

// GlobalLogger is for the application main() to set up as a base
// TODO: this should be unexported and a helper function set up to set this
var GlobalLogger Logger

// MustLogger will return a logger or will panic
func mustLogger(module string) Logger {
	if GlobalLogger == nil {
		panic("Global Logger is not set up, cannot return logger.")
	}
	return With(GlobalLogger, "module", module)
}

// WithRequestID returns a context with embedded request ID
func WithRequestID(ctx context.Context, requestID eh.UUID) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithSessionID returns a context with embedded request ID
func WithSessionID(ctx context.Context, sessionID eh.UUID) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// ContextLogger returns a logger and adds as much info as possible obtained from the context
func ContextLogger(ctx context.Context, module string, opts ...interface{}) Logger {
	newLogger := mustLogger(module)
	if ctx != nil {
		ctxRqID := ctx.Value(requestIDKey)
		if ctxRqID != nil {
			newLogger = With(newLogger, "requestID", ctxRqID)
		}

		ctxSessionID := ctx.Value(sessionIDKey)
		if ctxSessionID != nil {
			newLogger = With(newLogger, "sessionID", ctxSessionID)
		}
	}
	if len(opts) > 0 {
		newLogger = With(newLogger, opts...)
	}
	return newLogger
}
