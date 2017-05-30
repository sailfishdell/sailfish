package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
)

func main() {
	var (
		listen   = flag.String("listen", ":8080", "HTTP listen address")
		rootpath = flag.String("root", "serve", "base path from which to serve redfish data templates")
	)
	flag.Parse()

	var logger log.Logger
	logger = log.NewLogfmtLogger(os.Stderr)
	logger = log.With(logger, "listen", *listen, "caller", log.DefaultCaller)

/*
    // START prometheus metrics
    fieldKeys := []string{"method", "error"}
    requestCount := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
        Namespace: "redfish_group",
        Subsystem: "redfish_service",
        Name:      "request_count",
        Help:      "Number of requests received.",
    }, fieldKeys)
    requestLatency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
        Namespace: "redfish_group",
        Subsystem: "redfish_service",
        Name:      "request_latency_microseconds",
        Help:      "Total duration of requests in microseconds.",
    }, fieldKeys)
    countResult := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
        Namespace: "redfish_group",
        Subsystem: "redfish_service",
        Name:      "count_result",
        Help:      "The result of each count method.",
    }, []string{})
    // END prometheus metrics
*/

	var svc RedfishService
	svc = NewService(*rootpath, logger)
    svc = NewLoggingService(log.With(logger, "foo", "bar"), svc)
	redfishHandler := httptransport.NewServer(
		makeRedfishEndpoint(svc),
		decodeRedfishRequest,
		encodeResponse,
	)

	r := mux.NewRouter()
	r.PathPrefix("/redfish/v1/").Handler(http.StripPrefix("/redfish/v1/", redfishHandler))

	http.Handle("/", r)
	logger.Log("msg", "HTTP", "addr", *listen)
	logger.Log("err", http.ListenAndServe(*listen, nil))
}
