package redfishserver

import (
	"context"
	"github.com/go-kit/kit/endpoint"
)

func makeRawJSONRedfishGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(templatedRedfishGetRequest)
		output, err := s.RawJSONRedfishGet(ctx, req.pathTemplate, req.url, req.args)
		return output, err
	}
}

type templatedRedfishGetRequest struct {
	headers map[string]string
	url     string
	args    map[string]string
    pathTemplate string
}
