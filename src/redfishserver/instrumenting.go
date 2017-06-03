package redfishserver

import (
	"context"
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

func (s *instrumentingService) RedfishGet(ctx context.Context, r *http.Request) (ret interface{}, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.RedfishGet(ctx, r)
}

func (s *instrumentingService) RedfishPut(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.RedfishPut(ctx, r)
}

func (s *instrumentingService) RedfishPatch(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.RedfishPatch(ctx, r)
}

func (s *instrumentingService) RedfishPost(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.RedfishPost(ctx, r)
}

func (s *instrumentingService) RedfishDelete(ctx context.Context, r *http.Request) (ret []byte, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.RedfishService.RedfishDelete(ctx, r)
}
