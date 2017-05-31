package server

import (
	"net/http"
	"time"

	"github.com/go-kit/kit/metrics"
)

type instrumentingService struct {
	requestCount   metrics.Counter
	requestLatency metrics.Histogram
	RedfishService
}

// NewInstrumentingService returns an instance of an instrumenting Service.
func NewInstrumentingService(counter metrics.Counter, latency metrics.Histogram, s RedfishService) RedfishService {
	return &instrumentingService{
		requestCount:   counter,
		requestLatency: latency,
		RedfishService: s,
	}
}

func (s *instrumentingService) GetRedfish(r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.GetRedfish(r)
}

func (s *instrumentingService) PutRedfish(r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.PutRedfish(r)
}

func (s *instrumentingService) PatchRedfish(r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.PatchRedfish(r)
}

func (s *instrumentingService) PostRedfish(r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.PostRedfish(r)
}

func (s *instrumentingService) DeleteRedfish(r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.DeleteRedfish(r)
}
