package redfishserver

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	"io"
	"net/http"
)

func makeRedfishGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(redfishResourceRequest)
		// TODO: need to add 2 level error return:
		//      level 1: business logic errors that get mapped to HTTP transport errors
		//      level 2: other errors that might need to involve circuit breakers tripping (client request timeouts inside business logic functions, for example)
		output, err := s.GetRedfishResource(ctx, req.headers, req.url, req.args, req.privileges)
		return redfishResourceResponse{output: output, err: err}, nil
	}
}

func makeRedfishHandlerEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(redfishRequest)
		// TODO: need to add 2 level error return:
		//      level 1: business logic errors that get mapped to HTTP transport errors
		//      level 2: other errors that might need to involve circuit breakers tripping (client request timeouts inside business logic functions, for example)
		output, err := s.RedfishResourceHandler(ctx, req.r, req.privileges)
		return redfishResourceResponse{output: output, err: err}, nil
	}
}

type redfishRequest struct {
	r          *http.Request
	privileges []string
}

type redfishResourceRequest struct {
	headers    map[string]string
	url        string
	args       map[string]string
	privileges []string
	body       io.ReadCloser
}

type redfishResourceResponse struct {
	output interface{}
	err    error
}

func (in redfishResourceResponse) error() (err error) {
	if in.err != nil {
		return in.err
	}
	return nil
}
