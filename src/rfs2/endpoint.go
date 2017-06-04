package redfishserver

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	"net/http"
)

type Endpoints struct {
    RedfishRootGetEndpoint              endpoint.Endpoint
    RedfishSessionServiceGetEndpoint    endpoint.Endpoint
}

func MakeServerEndpoints(s Service) Endpoints {
    return Endpoints{
            RedfishRootGetEndpoint:  makeRedfishRootGetEndpoint(s),
        }
}

func makeRedfishRootGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.RedfishGet(ctx, req)
		return resp, err
	}
}
