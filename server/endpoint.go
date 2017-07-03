package redfishserver

import (
	"context"
	"github.com/go-kit/kit/endpoint"
)

func makeRawJSONRedfishGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(templatedRedfishGetRequest)
		// TODO: need to add 2 level error return:
		//      level 1: business logic errors that get mapped to HTTP transport errors
		//      level 2: other errors that might need to involve circuit breakers tripping (client request timeouts inside business logic functions, for example)
		output, err := s.RawJSONRedfishGet(ctx, req.pathTemplate, req.url, req.args)
		return templatedRedfishGetResponse{output: output, err: err}, nil
	}
}

type templatedRedfishGetRequest struct {
	headers      map[string]string
	url          string
	args         map[string]string
	pathTemplate string
}

type templatedRedfishGetResponse struct {
	output interface{}
	err    error
}

func (in templatedRedfishGetResponse) error() (err error) {
	if in.err != nil {
		return in.err
	}
	return nil
}
