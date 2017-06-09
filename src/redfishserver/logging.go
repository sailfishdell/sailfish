package redfishserver

import (
	"context"
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/google/uuid"
)

// Logger interface cut/paste from go-kit logging interface so that we don't have to have this dep everywhere
type Logger interface {
	Log(keyvals ...interface{}) error
}

type loggingService struct {
	logger Logger
	Service
}

type loggingIDType int

const (
	loggerKey loggingIDType = iota
)

// WithLogger returns a context which has a request-scoped logger
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// RequestLogger returns a structured logger with as much context as possible
func RequestLogger(ctx context.Context) Logger {
	var logger Logger
	if ctx != nil {
		if newLogger, ok := ctx.Value(loggerKey).(Logger); ok {
			logger = newLogger
		}
	}
	if logger == nil {
		logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	}
	return logger
}

// NewLoggingService returns a new instance of a logging Service.
func NewLoggingService(logger Logger, s Service) Service {
	return &loggingService{logger, s}
}

func (s *loggingService) RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (ret interface{}, err error) {
	ctxlogger := log.With(s.logger, "method", "GET", "URL", url, "UUID", uuid.New())

	thislogger := ctxlogger
	for k, v := range args {
		thislogger = log.With(thislogger, "arg_"+k, v)
	}
	defer func(begin time.Time) {
		// no, we are not going to check logger error return
		_ = thislogger.Log(
			"method", "GET",
			"URL", url,
			"PathTemplate", pathTemplate,
			"business_logic_time", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.Service.RawJSONRedfishGet(WithLogger(ctx, ctxlogger), pathTemplate, url, args)
}
