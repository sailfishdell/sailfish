package redfishserver

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	"net/http"
)

type Endpoints struct {
    RedfishSessionServiceGetEndpoint  endpoint.Endpoint
}

func makeRedfishGetEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.RedfishGet(ctx, req)
		return resp, err
	}
}

func makeRedfishPutEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.RedfishPut(ctx, req)
		return resp, err
	}
}

func makeRedfishPostEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.RedfishPost(ctx, req)
		return resp, err
	}
}

func makeRedfishPatchEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.RedfishPatch(ctx, req)
		return resp, err
	}
}

func makeRedfishDeleteEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.RedfishDelete(ctx, req)
		return resp, err
	}
}

