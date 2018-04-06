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

// WithRequestId returns a context with embedded request ID
func WithRequestId(ctx context.Context, requestId eh.UUID) context.Context {
	return context.WithValue(ctx, requestIdKey, requestId)
}

// WithSessionId returns a context with embedded request ID
func WithSessionId(ctx context.Context, sessionId eh.UUID) context.Context {
	return context.WithValue(ctx, sessionIdKey, sessionId)
}

// Logger returns a zap logger with as much context as possible
func ContextLogger(ctx context.Context, module string, opts ...interface{}) log.Logger {
	newLogger := log.MustLogger(module)
	if ctx != nil {
		ctxRqId := ctx.Value(requestIdKey)
		newLogger = newLogger.New("requestId", ctxRqId)

		ctxSessionId := ctx.Value(sessionIdKey)
		newLogger = newLogger.New("sessionId", ctxSessionId)
	}
	if len(opts) > 0 {
		newLogger = newLogger.New(opts...)
	}
	return newLogger
}
