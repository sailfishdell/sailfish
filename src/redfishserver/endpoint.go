package redfishserver

import (
	"context"
	"github.com/go-kit/kit/endpoint"
)

type Endpoints struct {
	RedfishRootGetEndpoint           endpoint.Endpoint
	RedfishSessionServiceGetEndpoint endpoint.Endpoint
}

func MakeServerEndpoints(s Service) Endpoints {
	return Endpoints{
		RedfishRootGetEndpoint: makeRedfishRootGetEndpoint(s),
	}
}

func makeRedfishRootGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(redfishGetRequest)
		output, err := s.RedfishGet(ctx, req.url)
		return output, err
	}
}

type redfishGetRequest struct {
	headers map[string]string
	url     string
}
