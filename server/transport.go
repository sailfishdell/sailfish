package redfishserver

import (
	"context"
	"encoding/json"
	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"net/http"
)

const serverHTTPHeader = "go-redfish/0.1"

// NewRedfishHandler is a function to hook up the redfish service to an http handler, it returns a mux
func NewRedfishHandler(svc Service, baseURI string, verURI string, logger log.Logger) http.Handler {
	r := mux.NewRouter()
	options := []httptransport.ServerOption{
		httptransport.ServerAfter(httptransport.SetContentType("application/json;charset=utf-8")),
		httptransport.ServerAfter(httptransport.SetResponseHeader("OData-Version", "4.0")),
		httptransport.ServerAfter(httptransport.SetResponseHeader("Server", serverHTTPHeader)),
		httptransport.ServerErrorLogger(logger),
		httptransport.ServerErrorEncoder(encodeError),
	}

	r.Handle(baseURI, http.RedirectHandler(baseURI+"/", 308))
	r.Handle(baseURI+"/"+verURI, http.RedirectHandler(baseURI+"/"+verURI+"/", 308))

	// READ SIDE STUFF
	r.Methods("GET").PathPrefix(baseURI + "/").Handler(
		httptransport.NewServer(
			makeRedfishGetEndpoint(svc),
			passthroughRequest,
			encodeResponse,
			options...,
		))
	/* not ready yet
	r.Methods("HEAD").PathPrefix(baseURI + "/").Handler(
		httptransport.NewServer(
			makeRedfishHandlerEndpoint(svc),
			passthroughRequest,
			encodeResponse,
			options...,
		))
	*/

	// COMMAND SIDE STUFF
	r.Methods("POST").PathPrefix(baseURI + "/").Handler(
		httptransport.NewServer(
			makeRedfishHandlerEndpoint(svc),
			passthroughRequest,
			encodeResponse,
			options...,
		))
	r.Methods("PUT").PathPrefix(baseURI + "/").Handler(
		httptransport.NewServer(
			makeRedfishHandlerEndpoint(svc),
			passthroughRequest,
			encodeResponse,
			options...,
		))
	r.Methods("PATCH").PathPrefix(baseURI + "/").Handler(
		httptransport.NewServer(
			makeRedfishHandlerEndpoint(svc),
			passthroughRequest,
			encodeResponse,
			options...,
		))
	r.Methods("DELETE").PathPrefix(baseURI + "/").Handler(
		httptransport.NewServer(
			makeRedfishHandlerEndpoint(svc),
			passthroughRequest,
			encodeResponse,
			options...,
		))

	return r
}

func checkHeaders(header http.Header) (err error) {
	// TODO: check Content-Type (for things with request body only)
	// TODO: check OData-MaxVersion "Indicates the maximum version of OData
	//                              that an odata-aware client understands"
	// TODO: check OData-Version "Services shall reject requests which specify
	//                              an unsupported OData version. If a service
	//                              encounters a version that it does not
	//                              support, the service should reject the
	//                              request with status code [412]
	//                              (#status-412). If client does not specify
	//                              an Odata-Version header, the client is
	//                              outside the boundaries of this
	//                              specification."

	return
}

func passthroughRequest(_ context.Context, r *http.Request) (dec interface{}, err error) {
	err = checkHeaders(r.Header)
	if err != nil {
		return nil, err
	}

	return redfishRequest{r: r, privileges: []string{"Unauthenticated"}}, nil
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
