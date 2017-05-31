package server

import (
	"net/http"
	"time"
    "context"

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

func (s *instrumentingService) GetRedfish(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.GetRedfish(ctx, r)
}

func (s *instrumentingService) PutRedfish(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.PutRedfish(ctx, r)
}

func (s *instrumentingService) PatchRedfish(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.PatchRedfish(ctx, r)
}

func (s *instrumentingService) PostRedfish(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.PostRedfish(ctx, r)
}

func (s *instrumentingService) DeleteRedfish(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.DeleteRedfish(ctx, r)
}
