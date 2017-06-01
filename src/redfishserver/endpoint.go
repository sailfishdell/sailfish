package redfishserver

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

func NewRedfishHandler(svc RedfishService, r *mux.Router) {
	r.PathPrefix("/redfish/v1/").Methods("GET").Handler(
		httptransport.NewServer(
			makeGetRedfishEndpoint(svc),
			decodeRedfishRequest,
			encodeResponse,
		))

	r.PathPrefix("/redfish/v1/").Methods("PUT").Handler(
		httptransport.NewServer(
			makePutRedfishEndpoint(svc),
			decodeRedfishRequest,
			encodeResponse,
		))

	r.PathPrefix("/redfish/v1/").Methods("POST").Handler(
		httptransport.NewServer(
			makePostRedfishEndpoint(svc),
			decodeRedfishRequest,
			encodeResponse,
		))

	r.PathPrefix("/redfish/v1/").Methods("PATCH").Handler(
		httptransport.NewServer(
			makePatchRedfishEndpoint(svc),
			decodeRedfishRequest,
			encodeResponse,
		))

	r.PathPrefix("/redfish/v1/").Methods("DELETE").Handler(
		httptransport.NewServer(
			makeDeleteRedfishEndpoint(svc),
			decodeRedfishRequest,
			encodeResponse,
		))
}
