package arbridge

import (
	"context"
	"encoding/json"
	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"net/http"
)

// NewHandler is a function to hook up the redfish service to an http handler, it returns a mux
func NewHandler(svc Service, baseURI string, logger log.Logger) http.Handler {

	r := mux.NewRouter()
	options := []httptransport.ServerOption{
		httptransport.ServerErrorLogger(logger),
		httptransport.ServerErrorEncoder(encodeError),
	}

	// COMMAND SIDE STUFF
	r.Methods("POST").PathPrefix(baseURI + "/").Handler(
		httptransport.NewServer(
			makeHandlerEndpoint(svc),
			passthroughRequest,
			encodeResponse,
			options...,
		))

	return r
}

func passthroughRequest(_ context.Context, r *http.Request) (dec interface{}, err error) {
	return r, nil
}

// probably could do something cool with channels and goroutines here so that
// we dont buffer the entire response, but not worth the effort at this moment
func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	decoded := response.(*Response)

	for k, v := range decoded.Headers {
		w.Header().Set(k, v)
	}

	if decoded.StatusCode != 0 {
		w.WriteHeader(decoded.StatusCode)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	switch output := decoded.Output.(type) {
	// can add case here to handle streaming output, if needed
	case []byte:
		_, err := w.Write(output)
		return err
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}
}

func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError with nil error")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codeFrom(err))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

func codeFrom(err error) int {
	switch err {
	case ErrNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
