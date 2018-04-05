package domain

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/src/log"
)

type requestIdType int

const (
	requestIdKey requestIdType = iota
	sessionIdKey
)

func init() {
	// a fallback/root logger for events without context
}

// WithRequestId returns a context with embedded request ID
func WithRequestId(ctx context.Context, requestId eh.UUID) context.Context {
	return context.WithValue(ctx, requestIdKey, requestId)
}

// WithSessionId returns a context with embedded request ID
func WithSessionId(ctx context.Context, sessionId eh.UUID) context.Context {
	return context.WithValue(ctx, sessionIdKey, sessionId)
}

// Logger returns a zap logger with as much context as possible
func ContextLogger(ctx context.Context, module string) log.Logger {
	newLogger := log.MustLogger(module)
	if ctx != nil {
		if ctxRqId, ok := ctx.Value(requestIdKey).(string); ok {
			newLogger = newLogger.New("requestId", ctxRqId)
		}
		if ctxSessionId, ok := ctx.Value(sessionIdKey).(string); ok {
			newLogger = newLogger.New("sessionId", ctxSessionId)
		}
	}
	return newLogger
}
