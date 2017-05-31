package server

import (
	"net/http"
	"time"
    "context"

	"github.com/go-kit/kit/log"
)

type loggingService struct {
	logger log.Logger
	RedfishService
}

// NewLoggingService returns a new instance of a logging Service.
func NewLoggingService(logger log.Logger, s RedfishService) RedfishService {
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
	return s.RedfishService.GetRedfish(ctx, r)
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
	return s.RedfishService.PutRedfish(ctx, r)
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
	return s.RedfishService.PatchRedfish(ctx, r)
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
	return s.RedfishService.PostRedfish(ctx, r)
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
	return s.RedfishService.DeleteRedfish(ctx, r)
}
