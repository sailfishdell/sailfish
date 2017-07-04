package redfishserver

import (
	"context"
	"github.com/go-kit/kit/endpoint"
)

func makeOdataGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(odataResourceGetRequest)
		// TODO: need to add 2 level error return:
		//      level 1: business logic errors that get mapped to HTTP transport errors
		//      level 2: other errors that might need to involve circuit breakers tripping (client request timeouts inside business logic functions, for example)
		output, err := s.GetOdataResource(ctx, req.headers, req.url, req.args, req.privileges)
		return odataResourceGetResponse{output: output, err: err}, nil
	}
}

type odataResourceGetRequest struct {
	headers    map[string]string
	url        string
	args       map[string]string
	privileges []string
}

type odataResourceGetResponse struct {
	output interface{}
	err    error
}

func (in odataResourceGetResponse) error() (err error) {
	if in.err != nil {
		return in.err
	}
	return nil
}
