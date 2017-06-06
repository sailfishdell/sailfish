package redfishserver

import (
	"context"
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

func (s *instrumentingService) RawJSONRedfishGet(ctx context.Context, templatePath, url string, args map[string]string) (interface{}, error) {
	defer func(begin time.Time) {
		s.requestCount.With("URL", url, "method", "GET").Add(1)
		s.requestLatency.With("URL", url, "method", "GET").Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.Service.RawJSONRedfishGet(ctx, templatePath, url, args)
}
