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
	Service
}

// NewInstrumentingService returns an instance of an instrumenting Service.
func NewInstrumentingService(counter metrics.Counter, latency metrics.Histogram, s Service) Service {
	return &instrumentingService{
		requestCount:   counter,
		requestLatency: latency,
		Service:        s,
	}
}

func (s *instrumentingService) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.Service.GetRedfishResource(ctx, r, privileges)
}

func (s *instrumentingService) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", r.URL.Path, "method", r.Method).Add(1)
		s.requestLatency.With("URL", r.URL.Path, "method", r.Method).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.Service.RedfishResourceHandler(ctx, r, privileges)
}
