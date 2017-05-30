package main

import (
    "time"
    "net/http"

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
        s.requestCount.With("method", "getRedfish").Add(1)
        s.requestLatency.With("method", "getRedfish").Observe(time.Since(begin).Seconds())
    }(time.Now())
    return s.RedfishService.GetRedfish(r)
}
