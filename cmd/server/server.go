package main

import (
	"context"
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

	eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	eventbus "github.com/superchalupa/eventhorizon/eventbus/local"
	eventstore "github.com/superchalupa/eventhorizon/eventstore/memory"
	eventpublisher "github.com/superchalupa/eventhorizon/publisher/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"

	"github.com/superchalupa/go-rfs/domain"
	redfishserver "github.com/superchalupa/go-rfs/server"

	_ "github.com/superchalupa/go-rfs/stdredfish"
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

	///
	// setup DDD stuff
	///

	// Create the event store.
	eventStore := eventstore.NewEventStore()

	// Create the event bus that distributes events.
	eventBus := eventbus.NewEventBus()
	//eventBus.SetHandlingStrategy( eh.AsyncEventHandlingStrategy )
	eventPublisher := eventpublisher.NewEventPublisher()
	//eventPublisher.SetHandlingStrategy( eh.AsyncEventHandlingStrategy )
	eventBus.SetPublisher(eventPublisher)

	// Create the command bus.
	commandBus := commandbus.NewCommandBus()

	// Create the read repositories.
	redfishRepo := repo.NewRepo()

	// Setup the domain.
	treeID := eh.NewUUID()
	waiter := domain.Setup(
		eventStore,
		eventBus,
		eventPublisher,
		commandBus,
		redfishRepo,
		treeID,
	)
	//
	// Done with DDD
	//

	logger = log.With(logger, "caller", log.DefaultCaller)

	svc := redfishserver.NewService(*baseUri, commandBus, eventBus, redfishRepo, treeID, waiter)
	// Need this *before* the authentication, so that the authentication module
	// will call this with the correct set of privileges
	svc = redfishserver.NewPrivilegeEnforcingService(svc, *baseUri, commandBus, redfishRepo, treeID)
	// Stack this *after* authorization so that it can get user info first and
	// pass privileges
	svc = redfishserver.NewBasicAuthService(svc, commandBus, redfishRepo, treeID, *baseUri)

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

	svc = redfishserver.NewLoggingService(logger, svc)

	r := redfishserver.NewRedfishHandler(svc, *baseUri, "v1", logger)

	http.Handle("/", r)
	http.Handle("/metrics", promhttp.Handler())

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
				logger.Log("err", srv.ListenAndServe())
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
				logger.Log("err", fcgi.Serve(listener, r))
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
