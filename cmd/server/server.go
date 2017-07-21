package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-yaml/yaml"
	//	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	//	stdprometheus "github.com/prometheus/client_golang/prometheus"
	//	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"os/signal"
	"strings"

	"github.com/superchalupa/go-redfish/domain"
	redfishserver "github.com/superchalupa/go-redfish/server"

	_ "github.com/superchalupa/go-redfish/provider/session"
	"net/http/pprof"
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
		baseURI     = flag.String("redfish_base_uri", "/redfish", "http base uri")
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

	///
	// setup DDD stuff
	///
	ddd := domain.BaseDDDFactory(*baseURI, "v1")
	domain.Setup(ddd)

	logger = log.With(logger, "caller", log.DefaultCaller)

	/*
	       // instrumentation setup
	   	fieldKeys := []string{"method", "URL"}
	       count := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
	           Namespace: "redfish",
	           Subsystem: "redfish_service",
	           Name:      "request_count",
	           Help:      "Number of requests received.",
	       }, fieldKeys)
	       lat := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
	           Namespace: "redfish",
	           Subsystem: "redfish_service",
	           Name:      "request_latency_microseconds",
	           Help:      "Total duration of requests in microseconds.",
	       }, fieldKeys)
	*/

	// ingest from our source
	// Ingest will recursively ingest from the source
	go redfishserver.Ingest(redfishserver.NewSPMFIngester("template/SPMF"), ddd, *baseURI+"/v1/")

	svc := redfishserver.NewService(ddd)
	svc = redfishserver.NewPrivilegeEnforcingService(svc)
	svc = redfishserver.NewBasicAuthService(svc)
	svc = redfishserver.NewXAuthTokenService(svc)
	//	svc = redfishserver.NewInstrumentingService(count, lat, svc)
	svc = redfishserver.NewLoggingService(logger, svc)

	r := redfishserver.NewRedfishHandler(svc, *baseURI, "v1", logger)
	m := http.NewServeMux()
	m.Handle("/", r)
	//	m.Handle("/metrics", promhttp.Handler())
	m.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	m.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	m.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	m.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	m.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	servers := []*http.Server{}

	for _, listen := range cfg.Listen {
		var listener net.Listener
		var err error
		logger.Log("msg", "processing listen request for "+listen)
		switch {
		case strings.HasPrefix(listen, "fcgi:") && strings.Contains(strings.TrimPrefix(listen, "fcgi:"), ":"):
			// FCGI listener on a TCP socket (usually should be specified as 127.0.0.1 for security)  fcgi:127.0.0.1:4040
			addr := strings.TrimPrefix(listen, "fcgi:")
			logger.Log("msg", "FCGI mode activated with tcp listener: "+addr)
			listener, err = net.Listen("tcp", addr)

		case strings.HasPrefix(listen, "fcgi:") && strings.Contains(strings.TrimPrefix(listen, "fcgi:"), "/"):
			// FCGI listener on unix domain socket, specified as a path fcgi:/run/fcgi.sock
			path := strings.TrimPrefix(listen, "fcgi:")
			logger.Log("msg", "FCGI mode activated with unix socket listener: "+path)
			listener, err = net.Listen("unix", path)
			defer os.Remove(path)

		case strings.HasPrefix(listen, "fcgi:"):
			// FCGI listener using stdin/stdout  fcgi:
			logger.Log("msg", "FCGI mode activated with stdin/stdout listener")
			listener = nil

		case strings.HasPrefix(listen, "http:"):
			// HTTP protocol listener
			addr := strings.TrimPrefix(listen, "http:")
			logger.Log("msg", "HTTP listener starting on "+addr)
			srv := &http.Server{Addr: addr}
			servers = append(servers, srv)
			go func(listen string) {
				logger.Log("err", http.ListenAndServe(addr, m))
			}(listen)

		case strings.HasPrefix(listen, "https:"):
			// HTTPS protocol listener
			// "https:[addr]:port,certfile,keyfile
			addr := strings.TrimPrefix(listen, "https:")
			details := strings.SplitN(addr, ",", 3)
			logger.Log("msg", "HTTPS listener starting on "+details[0], "certfile", details[1], "keyfile", details[2])
			srv := &http.Server{Addr: details[0]}
			servers = append(servers, srv)
			go func(listen string) {
				logger.Log("err", srv.ListenAndServeTLS(details[1], details[2]))
			}(listen)

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
				logger.Log("err", fcgi.Serve(listener, m))
			}(listener)
		}
	}

	fmt.Printf("%v\n", listenAddrs)

	<-intr
	fmt.Printf("interrupted\n")

	type Shutdowner interface {
		Shutdown(context.Context) error
	}

	for _, srv := range servers {
		// go 1.7 doesn't have Shutdown method on http server, so optionally cast
		// the interface to see if it exists, then call it, if possible Can
		// only do this with interfaces not concrete structs, so define a func
		// taking needed interface and call it.
		func(srv interface{}, addr string) {
			fmt.Println("msg", "shutting down listener: "+addr)
			if s, ok := srv.(Shutdowner); ok {
				logger.Log("msg", "shutting down listener: "+addr)
				if err := s.Shutdown(nil); err != nil {
					fmt.Println("server_error", err)
				}
			} else {
				fmt.Println("msg", "can't cleanly shutdown listener, it will ungracefully end: "+addr)
			}
		}(srv, srv.Addr)
	}

	fmt.Printf("Bye!\n")
}
