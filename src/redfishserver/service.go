package redfishserver

import (
	"context"
	"errors"
)

type Service interface {
	RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (interface{}, error)
}

// ServiceMiddleware is a chainable behavior modifier for Service.
type ServiceMiddleware func(Service) Service

type Config struct {
	// backend functions
	GetJSONOutput   func(context.Context, Logger, string, string, map[string]string) (interface{}, error)
	BackendUserdata interface{}
	logger          Logger
}

func NewService(logger Logger, rh Config) Service {
	return &rh
}

var (
	ErrNotFound = errors.New("not found")
)

func (rh *Config) RawJSONRedfishGet(ctx context.Context, pathTemplate, url string, args map[string]string) (interface{}, error) {
	logger := RequestLogger(ctx)
	//logger.Log("msg", "HELLO WORLD: rawjson")

	return rh.GetJSONOutput(ctx, logger, pathTemplate, url, args)
}
