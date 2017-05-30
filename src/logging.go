package main

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
			"method", "GET",
			"took", time.Since(begin),
			"err", err,
		)
	}(time.Now())
	return s.RedfishService.GetRedfish(r)
}
