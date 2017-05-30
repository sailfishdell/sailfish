package main

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	"net/http"
)

func decodeRedfishRequest(_ context.Context, r *http.Request) (interface{}, error) {
	// import "io/ioutil"
	//  return ioutil.ReadAll(r.Body)
	return r, nil
}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	s := response.([]byte)
	w.Write(s)
	return nil
}

func makeRedfishEndpoint(s RedfishService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*http.Request)
		resp, err := s.GetRedfish(req)
		return resp, err
	}
}
