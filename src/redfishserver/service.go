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
	logger    Logger
	pickleDir string

	// This holds all of our data
	odata map[string]interface{}
}

var (
	// ErrNotFound is returned when a request isnt present (404)
	ErrNotFound = errors.New("not found")
)

// NewService is how we initialize the business logic
func NewService(logger Logger, pickleDir string) Service {
	cfg := config{logger: logger, pickleDir: pickleDir, odata: map[string]interface{}{}}
	return &cfg
}

// Marhsaller interface is for objects that might want to do background stuff before the marshalling starts. After marshalling, MarshalJSON is the interface to get the data out
type Marshaller interface {
	StartMarshal(ctx context.Context, pathTemplate, url string, args map[string]string) (interface{}, error)
}

func (rh *config) RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (output interface{}, err error) {
	noHashPath := strings.SplitN(url, "#", 2)[0]
	r, ok := rh.odata[noHashPath]
	if !ok {
		return nil, ErrNotFound
	}

	switch r := r.(type) {
	case Marshaller:
		return r.StartMarshal(ctx, pathTemplate, url, args)
	}

	return r, nil
}
