package arbridge

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	"net/http"
)

func makeHandlerEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		return s.ResourceHandler(ctx, req, []string{})
	}
}
