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

func NewRedfishHandler(svc RedfishService, r *mux.Router) {
	contentTypeOption := httptransport.ServerAfter(httptransport.SetContentType("application/json;charset=utf-8"))
	odataVersion := httptransport.ServerAfter(httptransport.SetResponseHeader("OData-Version", "4.0"))
	// TODO: Content-Encoding: (should) - probably for gzip? doesn't look like its supported yet
	// TODO: Location - (CONDITIONAL SHALL) for POST to return where the object was created
	// TODO: Cache-Control
	// TODO: Via
	// TODO: Max-Forwards (SHOULD)
	// TODO: Access-Control-Allow-Origin (SHALL)
	// TODO: Allow - (SHALL) - returns GET/PUT/POST/PATCH/DELETE/HEAD

	r.PathPrefix("/redfish/v1/").Methods("GET").Handler(
		httptransport.NewServer(
			makeRedfishGetEndpoint(svc),
			decodeRedfishRequest,
			encodeResponse,
			contentTypeOption,
			odataVersion,
		))

	r.PathPrefix("/redfish/v1/").Methods("PUT").Handler(
		httptransport.NewServer(
			makeRedfishPutEndpoint(svc),
			decodeRedfishRequest,
			encodeResponse,
			contentTypeOption,
			odataVersion,
		))

	r.PathPrefix("/redfish/v1/").Methods("POST").Handler(
		httptransport.NewServer(
			makeRedfishPostEndpoint(svc),
			decodeRedfishRequest,
			encodeResponse,
			contentTypeOption,
			odataVersion,
		))

	r.PathPrefix("/redfish/v1/").Methods("PATCH").Handler(
		httptransport.NewServer(
			makeRedfishPatchEndpoint(svc),
			decodeRedfishRequest,
			encodeResponse,
			contentTypeOption,
			odataVersion,
		))

	r.PathPrefix("/redfish/v1/").Methods("DELETE").Handler(
		httptransport.NewServer(
			makeRedfishDeleteEndpoint(svc),
			decodeRedfishRequest,
			encodeResponse,
			contentTypeOption,
			odataVersion,
		))
}
