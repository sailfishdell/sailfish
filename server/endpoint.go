package redfishserver

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	"net/http"
)

func makeRedfishGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(redfishRequest)
		return s.GetRedfishResource(ctx, req.r, req.privileges)
	}
}

func makeRedfishHandlerEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(redfishRequest)
		return s.RedfishResourceHandler(ctx, req.r, req.privileges)
	}
}

type redfishRequest struct {
	r          *http.Request
	privileges []string
}
