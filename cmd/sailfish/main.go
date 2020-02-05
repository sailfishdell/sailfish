package main

import (
	"context"
	"crypto/elliptic"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/gorilla/mux"

	log "github.com/superchalupa/sailfish/src/log"
	applog "github.com/superchalupa/sailfish/src/log15adapter"

	"github.com/superchalupa/sailfish/src/http_redfish_sse"
	"github.com/superchalupa/sailfish/src/http_sse"
	"github.com/superchalupa/sailfish/src/httpinject"
	"github.com/superchalupa/sailfish/src/rawjsonstream"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	// cert gen
	"github.com/superchalupa/sailfish/src/tlscert"

	// load idrac plugins

	"github.com/superchalupa/sailfish/src/dell-resources/dellauth"
	"github.com/superchalupa/sailfish/src/ocp/basicauth"
	"github.com/superchalupa/sailfish/src/ocp/session"
)

type implementationFn func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.RWMutex, d *domain.DomainObjects) interface{}

var implementations = map[string]implementationFn{}

func main() {
	flag.StringSliceP("listen", "l", []string{}, "Listen address.  Formats: (http:[ip]:nn, https:[ip]:port)")

	var cfgMgrMu sync.RWMutex
	cfgMgr := viper.New()
	if err := cfgMgr.BindPFlags(flag.CommandLine); err != nil {
		fmt.Fprintf(os.Stderr, "Could not bind viper flags: %s\n", err)
		panic(fmt.Sprintf("Could not bind viper flags: %s", err))
	}
	// Environment variables
	cfgMgr.SetEnvPrefix("RF")
	cfgMgr.AutomaticEnv()

	// Configuration file
	cfgMgr.SetConfigName("redfish")
	cfgMgr.AddConfigPath(".")
	cfgMgr.AddConfigPath("/etc/")
	if err := cfgMgr.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not read config file: %s\n", err)
		panic(fmt.Sprintf("Could not read config file: %s", err))
	}

	// Local config for running from the build tree
	if fileExists("local-redfish.yaml") {
		fmt.Fprintf(os.Stderr, "Reading local-redfish.yaml config\n")
		cfgMgr.SetConfigFile("local-redfish.yaml")
		if err := cfgMgr.MergeInConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading local config file: %s\n", err)
			panic(fmt.Sprintf("Error reading local config file: %s", err))
		}
	}

	// Defaults
	cfgMgr.SetDefault("listen", []string{"https::8443"})
	cfgMgr.SetDefault("main.server_name", "mockup")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	logger := applog.InitializeApplicationLogging("")

	domainObjs, _ := domain.NewDomainObjects()
	// redo this later to observe events
	//domainObjs.EventPublisher.AddObserver(logger)
	domainObjs.CommandHandler = makeLoggingCmdHandler(logger, domainObjs.CommandHandler)

	// This also initializes all of the plugins
	domain.InitDomain(ctx, domainObjs.CommandHandler, domainObjs.EventBus, domainObjs.EventWaiter)

	// Handle the API.
	m := mux.NewRouter()
	loggingHTTPHandler := makeLoggingHTTPHandler(logger, m)

	injectSvc := httpinject.New(logger, domainObjs)
	injectSvc.Start()

	// per spec: hardcoded output for /redfish to list versions supported.
	m.Path("/redfish").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte("{\n\t\"v1\": \"/redfish/v1/\"\n}\n"))
	})
	// per spec: redirect /redfish/ to /redfish/v1
	m.Path("/redfish/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/redfish/v1", http.StatusMovedPermanently)
	})

	// some static files that we should generate at some point
	m.Path("/redfish/v1/$metadata").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "v1/metadata.xml") })
	m.Path("/redfish/v1/odata").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "v1/odata.json") })

	// generic handler for redfish output on most http verbs
	// Note: this works by using the session service to get user details from token to pass up the stack using the embedded struct
	chainAuth := func(u string, p []string) http.Handler { return domain.NewRedfishHandler(domainObjs, logger, u, p) }

	handlerFunc := dellauth.MakeHandlerFunc(chainAuth,
		session.MakeHandlerFunc(logger, domainObjs.EventBus, domainObjs, chainAuth,
			basicauth.MakeHandlerFunc(chainAuth,
				chainAuth("UNKNOWN", []string{"Unauthenticated"}))))

	// SSE
	chainAuthSSE := func(u string, p []string) http.Handler { return http_sse.NewSSEHandler(domainObjs, logger, u, p) }
	m.Path("/events").Methods("GET").HandlerFunc(
		session.MakeHandlerFunc(logger, domainObjs.EventBus, domainObjs, chainAuthSSE, basicauth.MakeHandlerFunc(chainAuthSSE, chainAuthSSE("UNKNOWN", []string{"Unauthenticated"}))))

	// Redfish SSE
	chainAuthRFSSE := func(u string, p []string) http.Handler {
		return http_redfish_sse.NewRedfishSSEHandler(domainObjs, logger, u, p)
	}
	m.Path("/redfish/v1/SSE").Methods("GET").HandlerFunc(
		session.MakeHandlerFunc(logger, domainObjs.EventBus, domainObjs, chainAuthRFSSE, basicauth.MakeHandlerFunc(chainAuthRFSSE, chainAuthRFSSE("UNKNOWN", []string{"Unauthenticated"}))))

	// backend command handling
	internalHandlerFunc := domainObjs.GetInternalCommandHandler(ctx)

	// most-used command is event inject, specify that manually to avoid some regexp memory allocations

	m.Path("/api/Event:Inject").Methods("POST").HandlerFunc(injectSvc.GetHandlerFunc())

	// All of the /redfish apis
	m.PathPrefix("/redfish/").Methods("GET", "PUT", "POST", "PATCH", "DELETE", "HEAD", "OPTIONS").HandlerFunc(handlerFunc)

	// serve up the schema XML
	m.PathPrefix("/schemas/v1/").Handler(http.StripPrefix("/schemas/v1/", http.FileServer(http.Dir("./v1/schemas/"))))

	// all the other command apis.
	m.PathPrefix("/api/{command}").Methods("POST").Handler(internalHandlerFunc)

	// debugging (localhost only)
	m.Path("/status").Handler(domainObjs.DumpStatus())

	// This starts goroutines that use cfgmgr, so from here on out we need to lock it
	implFn, ok := implementations[cfgMgr.GetString("main.server_name")]
	if !ok {
		panic("could not load implementation specified in main.server_name: " + cfgMgr.GetString("main.server_name"))
	}
	implFn(ctx, logger, cfgMgr, &cfgMgrMu, domainObjs)

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

	type shutdowner interface {
		Shutdown(context.Context) error
	}
	shutdownlist := []shutdowner{}
	addShutdown := func(name string, srv interface{}) {
		s, ok := srv.(shutdowner)
		if !ok {
			logger.Crit("The requested HTTP server can't be gracefully shutdown at program exit.", "ServerName", name)
			return
		}
		shutdownlist = append(shutdownlist, s)
	}

	// And finally, start up all of the listeners that we have configured
	cfgMgrMu.RLock()
	listeners := cfgMgr.GetStringSlice("listen")
	if len(listeners) == 0 {
		fmt.Fprintf(os.Stderr, "No listeners configured! Use the '-l' option to configure a listener!")
	}
	cfgMgrMu.RUnlock()

	fmt.Printf("Starting services: %s\n", listeners)
	for _, listen := range listeners {
		switch {
		case strings.HasPrefix(listen, "pprof:"):
			pprofListener := runpprof(logger, addShutdown, listen)
			go pprofListener()
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
			go func(listen string, s *http.Server) {
				logger.Crit("Server exited", "listenaddr", listen, "err", s.ListenAndServe())
			}(listen, s)
			addShutdown(listen, s)

		case strings.HasPrefix(listen, "unix:"):
			// HTTP protocol listener
			// "https:[addr]:port
			addr := strings.TrimPrefix(listen, "unix:")

			// delete old socket file
			if _, err := os.Stat(addr); !os.IsNotExist(err) {
				logger.Info("Socket file found, deleting...")
				err := os.Remove(addr)
				if err != nil {
					logger.Error("Could not remove old socket file", "Error", err.Error())
				}
			}

			logger.Info("UNIX SOCKET listener starting on " + addr)
			s := &http.Server{
				Handler:        loggingHTTPHandler,
				MaxHeaderBytes: 1 << 20,
				ReadTimeout:    100 * time.Second,
			}
			unixListener, err := net.Listen("unix", addr)
			if err != nil {
				break
			}
			go func(listen string, s *http.Server, l net.Listener) {
				logger.Crit("Server exited", "listenaddr", listen, "err", s.Serve(l))
			}(listen, s, unixListener)
			addShutdown(listen, s)

		case strings.HasPrefix(listen, "pipeinput:"):
			pipeName := strings.TrimPrefix(listen, "pipeinput:")
			go rawjsonstream.StartPipeHandler(logger, pipeName, domainObjs, injectSvc)
			logger.Crit("pipe listener started", "path", pipeName)

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
			go func(listen string, s *http.Server) {
				logger.Crit("Server exited", "listenaddr", listen, "err", s.ListenAndServeTLS("server.crt", "server.key"))
			}(listen, s)
			addShutdown(listen, s)
		}
	}

	logger.Debug("Listening", "module", "main", "addresses", fmt.Sprintf("%v\n", listeners))
	injectSvc.Ready()

	// start periodically forcing OS memory free
	go func() {
		t := time.Tick(time.Second * 30)
		for {
			<-t
			debug.FreeOSMemory()
		}
	}()

	// wait until we get an interrupt (CTRL-C)
	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)
	<-intr
	logger.Crit("INTERRUPTED, Cancelling...")
	cancel()

	// wait up to 1 second for active connections
	shutdownCtx, cancelshutdown := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelshutdown()
	for _, s := range shutdownlist {
		err := s.Shutdown(shutdownCtx)
		if err != nil {
			logger.Crit("server wasn't gracefully shutdown.", "err", err)
		}
	}
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

func fileExists(fn string) bool {
	fd, err := os.Stat(fn)
	if os.IsNotExist(err) {
		return false
	}
	return !fd.IsDir()
}
