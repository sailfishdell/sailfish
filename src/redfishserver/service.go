package redfishserver

import (
	"context"
	"errors"
	"strings"
)

// Service is the business logic for a redfish server
type Service interface {
	RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (interface{}, error)
	Startup() chan struct{}
}

// ServiceMiddleware is a chainable behavior modifier for Service.
type ServiceMiddleware func(Service) Service

// Config is where we store the current service data
type config struct {
	logger  Logger
	baseURI string
	verURI  string

	// This holds all of our data
	odata OdataTree
}

var (
	// ErrNotFound is returned when a request isnt present (404)
	ErrNotFound = errors.New("not found")
)

// NewService is how we initialize the business logic
func NewService(logger Logger, baseURI string) Service {
	cfg := config{logger: logger, baseURI: baseURI, verURI: "v1", odata: NewOdataTree()}
	return &cfg
}

func (rh *config) RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (output interface{}, err error) {
	noHashPath := strings.SplitN(url, "#", 2)[0]
	r, ok := rh.odata.GetBody(noHashPath)
	if !ok {
		return nil, ErrNotFound
	}

	return r.OdataSerialize(ctx)
}
