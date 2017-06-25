package main

import (
	"flag"
	"fmt"
	"github.com/go-kit/kit/log"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	"github.com/go-yaml/yaml"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"os/signal"
	"strings"

	"github.com/superchalupa/go-redfish/src/redfishserver"
)

type appConfig struct {
	Listen []string `yaml:"listen"`
}

func loadConfig(filename string) (appConfig, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return appConfig{}, err
	}

	var config appConfig
	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		return appConfig{}, err
	}

	return config, nil
}

// Define a type named "strslice" as a slice of ints
type strslice []string

// Now, for our new type, implement the two methods of
// the flag.Value interface...
// The first method is String() string
func (i *strslice) String() string {
	return fmt.Sprintf("%v", *i)
}

// The second method is Set(value string) error
func (i *strslice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	var (
		configFile  = flag.String("config", "app.yaml", "Application configuration file")
		baseUri     = flag.String("redfish_base_uri", "/redfish", "http base uri")
		listenAddrs strslice
	)
	flag.Var(&listenAddrs, "l", "Listen address.  Formats: (:nn, fcgi:ip:port, fcgi:/path)")
	flag.Parse()

	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)

	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))

	cfg, _ := loadConfig(*configFile)
	if len(listenAddrs) > 0 {
		cfg.Listen = listenAddrs
	}

	logger = log.With(logger, "listen", cfg.Listen, "caller", log.DefaultCaller)

	svc := redfishserver.NewService(logger, *baseUri)
	svc = redfishserver.NewLoggingService(logger, svc)

	fieldKeys := []string{"method", "URL"}
	svc = redfishserver.NewInstrumentingService(
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

	r := redfishserver.NewRedfishHandler(svc, *baseUri, "v1", logger)

	done := svc.Startup()
	defer close(done)

	http.Handle("/", r)
	http.Handle("/metrics", promhttp.Handler())

	for _, listen := range cfg.Listen {
		var listener net.Listener
		var err error
		logger.Log("msg", "processing listen request for "+listen)
		switch {
        // FCGI listener on a TCP socket (usually should be specified as 127.0.0.1 for security)  fcgi:127.0.0.1:4040
		case strings.HasPrefix(listen, "fcgi:") && strings.Contains(strings.TrimPrefix(listen, "fcgi:"), ":"):
			addr := strings.TrimPrefix(listen, "fcgi:")
			logger.Log("msg", "FCGI mode activated with tcp listener: "+addr)
			listener, err = net.Listen("tcp", addr)

        // FCGI listener on unix domain socket, specified as a path fcgi:/run/fcgi.sock
		case strings.HasPrefix(listen, "fcgi:") && strings.Contains(strings.TrimPrefix(listen, "fcgi:"), "/"):
			path := strings.TrimPrefix(listen, "fcgi:")
			logger.Log("msg", "FCGI mode activated with unix socket listener: "+path)
			listener, err = net.Listen("unix", path)
			defer os.Remove(path)

        // FCGI listener using stdin/stdout  fcgi:
		case strings.HasPrefix(listen, "fcgi:"):
			logger.Log("msg", "FCGI mode activated with stdin/stdout listener")
			listener = nil

        // HTTP protocol listener
		case strings.HasPrefix(listen, "http:"):
			addr := strings.TrimPrefix(listen, "http:")
			go func(listen string) {
				logger.Log("msg", "HTTP", "addr", addr)
				logger.Log("err", http.ListenAndServe(addr, nil))
			}(listen)
            // next listener, no need to do if() stuff below
            continue
		}

		if strings.HasPrefix(listen, "fcgi:") {
			if err != nil {
				logger.Log("fatal", "Could not open listening connection", "err", err)
				return
			}
			if listener != nil {
				defer listener.Close()
			}
			go func(listener net.Listener) {
				logger.Log("err", fcgi.Serve(listener, r))
			}(listener)
		}
	}

	fmt.Printf("%v\n", listenAddrs)

	<-intr

}
