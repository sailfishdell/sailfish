package redfishserver

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/google/uuid"
)

type Logger interface {
	Log(keyvals ...interface{}) error
}

type loggingService struct {
	logger Logger
	RedfishService
}

type loggingIdType int

const (
	loggerKey loggingIdType = iota
)

// WithLogger returns a context which has a request-scoped logger
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// Logger returns a structured logger with as much context as possible
func RequestLogger(ctx context.Context) Logger {
	var logger Logger = nil
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
func NewLoggingService(logger Logger, s RedfishService) RedfishService {
	return &loggingService{logger, s}
}

func (s *loggingService) GetRedfish(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.logger.Log(
			"method", r.Method,
			"URL", r.URL.Path,
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.GetRedfish(WithLogger(ctx, log.With(s.logger, "method", r.Method, "URL", r.URL.Path, "UUID", uuid.New())), r)
}

func (s *loggingService) PutRedfish(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.logger.Log(
			"method", r.Method,
			"URL", r.URL.Path,
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.PutRedfish(WithLogger(ctx, log.With(s.logger, "method", r.Method, "URL", r.URL.Path, "UUID", uuid.New())), r)
}

func (s *loggingService) PatchRedfish(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.logger.Log(
			"method", r.Method,
			"URL", r.URL.Path,
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.PatchRedfish(WithLogger(ctx, log.With(s.logger, "method", r.Method, "URL", r.URL.Path, "UUID", uuid.New())), r)
}

func (s *loggingService) PostRedfish(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.logger.Log(
			"method", r.Method,
			"URL", r.URL.Path,
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.PostRedfish(WithLogger(ctx, log.With(s.logger, "method", r.Method, "URL", r.URL.Path, "UUID", uuid.New())), r)
}

func (s *loggingService) DeleteRedfish(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.logger.Log(
			"method", r.Method,
			"URL", r.URL.Path,
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.DeleteRedfish(WithLogger(ctx, log.With(s.logger, "method", r.Method, "URL", r.URL.Path, "UUID", uuid.New())), r)
}
