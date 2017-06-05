package redfishserver

import (
	"context"
	"github.com/go-kit/kit/endpoint"
)

type Endpoints struct {
	RedfishRootGetEndpoint              endpoint.Endpoint
	RedfishV1RootGetEndpoint              endpoint.Endpoint
    RedfishSystemCollectionGetEndpoint  endpoint.Endpoint
}

func MakeServerEndpoints(s Service) Endpoints {
	return Endpoints{
		RedfishRootGetEndpoint: makeRawJSONRedfishGetEndpoint(s),
		RedfishV1RootGetEndpoint: makeTemplatedRedfishGetEndpoint(s, "V1Root.gojson"),
		RedfishSystemCollectionGetEndpoint: makeTemplatedRedfishGetEndpoint(s, "SystemCollection.gojson"),
	}
}

func makeTemplatedRedfishGetEndpoint(s Service, templateName string) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(templatedRedfishGetRequest)
		output, err := s.TemplatedRedfishGet(ctx, templateName, req.url, req.args)
		return output, err
	}
}

func makeRawJSONRedfishGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(templatedRedfishGetRequest)
		output, err := s.RawJSONRedfishGet(ctx, req.url, req.args)
		return output, err
	}
}

type templatedRedfishGetRequest struct {
	headers map[string]string
	url     string
	args    map[string]string
}
