package server

import (
	"net/http"
	"time"

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

func (s *loggingService) GetRedfish(r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.logger.Log(
			"method", r.Method,
			"URL", r.URL.Path,
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.GetRedfish(r)
}

func (s *loggingService) PutRedfish(r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.logger.Log(
			"method", r.Method,
			"URL", r.URL.Path,
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.PutRedfish(r)
}

func (s *loggingService) PatchRedfish(r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.logger.Log(
			"method", r.Method,
			"URL", r.URL.Path,
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.PatchRedfish(r)
}

func (s *loggingService) PostRedfish(r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.logger.Log(
			"method", r.Method,
			"URL", r.URL.Path,
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.PostRedfish(r)
}

func (s *loggingService) DeleteRedfish(r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.logger.Log(
			"method", r.Method,
			"URL", r.URL.Path,
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.DeleteRedfish(r)
}
