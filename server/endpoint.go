package redfishserver

import (
	"context"
	"github.com/go-kit/kit/endpoint"
)

func makeRedfishGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(redfishResourceGetRequest)
		// TODO: need to add 2 level error return:
		//      level 1: business logic errors that get mapped to HTTP transport errors
		//      level 2: other errors that might need to involve circuit breakers tripping (client request timeouts inside business logic functions, for example)
		output, err := s.GetRedfishResource(ctx, req.headers, req.url, req.args, req.privileges)
		return redfishResourceGetResponse{output: output, err: err}, nil
	}
}

type redfishResourceGetRequest struct {
	headers    map[string]string
	url        string
	args       map[string]string
	privileges []string
}

type redfishResourceGetResponse struct {
	output interface{}
	err    error
}

func (in redfishResourceGetResponse) error() (err error) {
	if in.err != nil {
		return in.err
	}
	return nil
}
