package main

import (
	"context"
	"crypto/elliptic"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/gorilla/mux"

	log "github.com/superchalupa/sailfish/src/log"
	applog "github.com/superchalupa/sailfish/src/log15adapter"

	"github.com/superchalupa/sailfish/src/http_sse"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	// cert gen
	"github.com/superchalupa/sailfish/src/tlscert"

	"github.com/superchalupa/sailfish/src/ocp/basicauth"
)

type implementationFn func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, d *domain.DomainObjects) Implementation

var implementations map[string]implementationFn = map[string]implementationFn{}

type Implementation interface {
}

func main() {
	flag.StringSliceP("listen", "l", []string{}, "Listen address.  Formats: (http:[ip]:nn, https:[ip]:port)")

	cfgMgr := viper.New()
	if err := cfgMgr.BindPFlags(flag.CommandLine); err != nil {
		fmt.Fprintf(os.Stderr, "Could not bind viper flags: %s\n", err)
	}
	// Environment variables
	cfgMgr.SetEnvPrefix("ME")
	cfgMgr.AutomaticEnv()

	// Configuration file
	cfgMgr.SetConfigName("metric-engine")
	cfgMgr.AddConfigPath("/etc/")
	cfgMgr.AddConfigPath(".")
	if err := cfgMgr.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not read config file: %s\n", err)
	}

	// Defaults
	cfgMgr.SetDefault("listen", []string{"https::8443"})
	cfgMgr.SetDefault("main.server_name", "idrac")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	logger := applog.InitializeApplicationLogging("metric-engine")

	domainObjs, _ := domain.NewDomainObjects()
	// redo this later to observe events
	//domainObjs.EventPublisher.AddObserver(logger)
	domainObjs.CommandHandler = makeLoggingCmdHandler(logger, domainObjs.CommandHandler)

	// This also initializes all of the plugins
	domain.InitDomain(ctx, domainObjs.CommandHandler, domainObjs.EventBus, domainObjs.EventWaiter)

	// Handle the API.
	m := mux.NewRouter()
	loggingHTTPHandler := makeLoggingHTTPHandler(logger, m)

	// SSE
	chainAuthSSE := func(u string, p []string) http.Handler { return http_sse.NewSSEHandler(domainObjs, logger, u, p) }
	m.Path("/events").
		Methods("GET").HandlerFunc(
		basicauth.MakeHandlerFunc(chainAuthSSE, chainAuthSSE("UNKNOWN", []string{"Unauthenticated"})))

	// backend command handling
	internalHandlerFunc := domainObjs.GetInternalCommandHandler(ctx)

	implFn, ok := implementations[cfgMgr.GetString("main.server_name")]
	if !ok {
		panic("could not load requested implementation: " + cfgMgr.GetString("main.server_name"))
	}

	// This starts goroutines that use cfgmgr, so from here on out we need to lock it
	implFn(ctx, logger, cfgMgr, domainObjs)

	// all the other command apis.
	m.PathPrefix("/api/{command}").Handler(internalHandlerFunc)

	// debugging (localhost only)
	m.Path("/status").Handler(domainObjs.DumpStatus())

	tlscfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// TODO: cli option to enable/disable
		// Secure, but way too slow
		CurvePreferences: []tls.CurveID{tls.CurveP256, tls.X25519, tls.CurveP384, tls.CurveP521},

		// TODO: cli option to enable/disable
		// Secure, but way too slow
		PreferServerCipherSuites: true,

		// TODO: cli option to enable/disable
		// Can't quite remember, but I think this breaks curl
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, // Go 1.8 only
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,   // Go 1.8 only
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	if len(cfgMgr.GetStringSlice("listen")) == 0 {
		fmt.Fprintf(os.Stderr, "No listeners configured! Use the '-l' option to configure a listener!")
	}

	fmt.Printf("Starting services: %s\n", cfgMgr.GetStringSlice("listen"))

	// And finally, start up all of the listeners that we have configured
	for _, listen := range cfgMgr.GetStringSlice("listen") {
		switch {
		case strings.HasPrefix(listen, "pprof:"):
			pprofMux := http.DefaultServeMux
			http.DefaultServeMux = http.NewServeMux()
			addr := strings.TrimPrefix(listen, "pprof:")
			s := &http.Server{
				Addr:    addr,
				Handler: pprofMux,
			}
			logger.Info("PPROF activated with tcp listener: " + addr)
			go s.ListenAndServe()
			go handleShutdown(ctx, logger, s)

		case strings.HasPrefix(listen, "http:"):
			// HTTP protocol listener
			// "https:[addr]:port
			addr := strings.TrimPrefix(listen, "http:")
			logger.Info("HTTP listener starting on " + addr)
			s := &http.Server{
				Addr:           addr,
				Handler:        loggingHTTPHandler,
				MaxHeaderBytes: 1 << 20,
				ReadTimeout:    100 * time.Second,
				// cannot use writetimeout if we are streaming
				// WriteTimeout:   10 * time.Second,
			}
			go func() { logger.Info("Server exited", "err", s.ListenAndServe()) }()
			go handleShutdown(ctx, logger, s)

		case strings.HasPrefix(listen, "unix:"):
			// HTTP protocol listener
			// "https:[addr]:port
			addr := strings.TrimPrefix(listen, "unix:")
			logger.Info("UNIX SOCKET listener starting on " + addr)
			s := &http.Server{
				Handler:        loggingHTTPHandler,
				MaxHeaderBytes: 1 << 20,
				ReadTimeout:    100 * time.Second,
			}
			unixListener, err := net.Listen("unix", addr)
			if err == nil {
				go func() { logger.Info("Server exited", "err", s.Serve(unixListener)) }()
				go handleShutdown(ctx, logger, s)
			}

		case strings.HasPrefix(listen, "https:"):
			// HTTPS protocol listener
			// "https:[addr]:port,certfile,keyfile
			addr := strings.TrimPrefix(listen, "https:")
			s := &http.Server{
				Addr:           addr,
				Handler:        loggingHTTPHandler,
				MaxHeaderBytes: 1 << 20,
				ReadTimeout:    10 * time.Second,
				// cannot use writetimeout if we are streaming
				// WriteTimeout:   10 * time.Second,
				TLSConfig: tlscfg,
				// can't remember why this doesn't work... TODO: reason this doesnt work
				//TLSNextProto:   make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
			}
			logger.Info("HTTPS listener starting on " + addr)
			checkCaCerts(logger)
			go func() { logger.Info("Server exited", "err", s.ListenAndServeTLS("server.crt", "server.key")) }()
			go handleShutdown(ctx, logger, s)

		}
	}

	logger.Debug("Listening", "module", "main", "addresses", fmt.Sprintf("%v\n", cfgMgr.GetStringSlice("listen")))
	SdNotify("READY=1")

	// wait until we get an interrupt (CTRL-C)
	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)
	<-intr
	logger.Crit("INTERRUPTED, Cancelling...")
	cancel()
	logger.Warn("Bye!", "module", "main")
}

func checkCaCerts(logger log.Logger) {
	// TODO: cli option to enable/disable and control cert options
	// Load CA cert if it exists. Create CA cert if it doesn't.
	ca, err := tlscert.Load(tlscert.SetBaseFilename("ca"), tlscert.WithLogger(logger))
	if err != nil {
		ca, _ = tlscert.NewCert(
			tlscert.CreateCA,
			tlscert.ExpireInOneYear,
			tlscert.SetCommonName("CA Cert common name"),
			tlscert.SetSerialNumber(12345),
			tlscert.SetBaseFilename("ca"),
			tlscert.GenECDSA(elliptic.P256()),
			tlscert.SelfSigned(),
			tlscert.WithLogger(logger),
		)
		ca.Serialize()
	}

	// TODO: cli option to enable/disable and control cert options
	// TODO: cli option to create new server cert unconditionally based on CA cert
	_, err = tlscert.Load(tlscert.SetBaseFilename("server"), tlscert.WithLogger(logger))
	if err != nil {
		serverCert, _ := tlscert.NewCert(
			tlscert.GenECDSA(elliptic.P256()),
			tlscert.SignWithCA(ca),
			tlscert.MakeServer,
			tlscert.ExpireInOneYear,
			tlscert.SetCommonName("localhost"),
			tlscert.SetSubjectKeyID([]byte{1, 2, 3, 4, 6}),
			tlscert.AddSANDNSName("localhost", "localhost.localdomain"),
			tlscert.SetSerialNumber(12346),
			tlscert.SetBaseFilename("server"),
			tlscert.WithLogger(logger),
		)
		iterInterfaceIPAddrs(logger, func(ip net.IP) { serverCert.ApplyOption(tlscert.AddSANIP(ip)) })
		serverCert.Serialize()
	}
}

type shutdowner interface {
	Shutdown(context.Context) error
}

func handleShutdown(ctx context.Context, logger log.Logger, srv interface{}) error {
	s, ok := srv.(shutdowner)
	if !ok {
		logger.Info("Can't cleanly shutdown listener, it will ungracefully end")
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("shutdown server.")
			if err := s.Shutdown(ctx); err != nil {
				logger.Info("server_error", "err", err)
			}
			return ctx.Err()
		}
	}
}

func iterInterfaceIPAddrs(logger log.Logger, fn func(net.IP)) {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			logger.Debug("Adding local IP Address to server cert as SAN", "ip", ip, "module", "main")
			fn(ip)
		}
	}
}

func init() {
	go func() {
		t := time.Tick(time.Second * 30)
		for {
			<-t
			debug.FreeOSMemory()
		}
	}()
}
