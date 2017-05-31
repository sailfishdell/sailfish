package server

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"net/http"
)

// we are basically tied to HTTP, so just pass the request down to the function
// don't anticipate ever adding grpc or other support here, so this should be fine for now
// if we do add, we'll have to simply re-work the function parameters.
func decodeRedfishRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return r, nil
}

// probably could do something cool with channels and goroutines here so that
// we dont buffer the entire response, but not worth the effort at this moment
func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	s := response.([]byte)
	w.Write(s)
	return nil
}

func makeGetRedfishEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.GetRedfish(ctx, req)
		return resp, err
	}
}

func makePutRedfishEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.PutRedfish(ctx, req)
		return resp, err
	}
}

func makePostRedfishEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.PostRedfish(ctx, req)
		return resp, err
	}
}

func makePatchRedfishEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.PatchRedfish(ctx, req)
		return resp, err
	}
}

func makeDeleteRedfishEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.DeleteRedfish(ctx, req)
		return resp, err
	}
}

// probably a better way to do this instead of hardcoding stuff here
func NewRedfishHandler(svc RedfishService, r *mux.Router) {
	getHandler := httptransport.NewServer(
		makeGetRedfishEndpoint(svc),
		decodeRedfishRequest,
		encodeResponse,
	)

	r.PathPrefix("/redfish/v1/").Handler(http.StripPrefix("/redfish/v1/", getHandler)).Methods("GET")

	putHandler := httptransport.NewServer(
		makePutRedfishEndpoint(svc),
		decodeRedfishRequest,
		encodeResponse,
	)

	r.PathPrefix("/redfish/v1/").Handler(http.StripPrefix("/redfish/v1/", putHandler)).Methods("PUT")

	postHandler := httptransport.NewServer(
		makePostRedfishEndpoint(svc),
		decodeRedfishRequest,
		encodeResponse,
	)

	r.PathPrefix("/redfish/v1/").Handler(http.StripPrefix("/redfish/v1/", postHandler)).Methods("POST")

	patchHandler := httptransport.NewServer(
		makePatchRedfishEndpoint(svc),
		decodeRedfishRequest,
		encodeResponse,
	)

	r.PathPrefix("/redfish/v1/").Handler(http.StripPrefix("/redfish/v1/", patchHandler)).Methods("PATCH")

	deleteHandler := httptransport.NewServer(
		makeDeleteRedfishEndpoint(svc),
		decodeRedfishRequest,
		encodeResponse,
	)

	r.PathPrefix("/redfish/v1/").Handler(http.StripPrefix("/redfish/v1/", deleteHandler)).Methods("DELETE")

}
