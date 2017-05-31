package main

import (
	"flag"
	"github.com/go-kit/kit/log"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	"github.com/gorilla/mux"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/superchalupa/go-redfish/src/server"
	"net/http"
	"os"
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

	var svc server.RedfishService
	svc = server.NewService(*rootpath, logger)
	svc = server.NewLoggingService(logger, svc)

	fieldKeys := []string{"method", "URL"}
	svc = server.NewInstrumentingService(
		kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "redfish",
			Subsystem: "redfish_service",
			Name:      "request_count",
			Help:      "Number of requests received.",
		}, fieldKeys),
		kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "redfish",
			Subsystem: "redfish_service",
			Name:      "request_latency_microseconds",
			Help:      "Total duration of requests in microseconds.",
		}, fieldKeys),
		svc,
	)

	r := mux.NewRouter()
	server.NewRedfishHandler(svc, r)

	http.Handle("/", r)
	http.Handle("/metrics", promhttp.Handler())

	logger.Log("msg", "HTTP", "addr", *listen)
	logger.Log("err", http.ListenAndServe(*listen, nil))
}
