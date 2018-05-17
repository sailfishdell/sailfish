package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/fcgi"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/gorilla/mux"
	log "github.com/superchalupa/go-redfish/src/log"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	// cert gen
	"github.com/superchalupa/go-redfish/src/tlscert"

	// load plugins (auto-register)
	"github.com/superchalupa/go-redfish/src/actionhandler"
	"github.com/superchalupa/go-redfish/src/stdcollections"
	_ "github.com/superchalupa/go-redfish/src/stdmeta"

	// load idrac plugins
	idrac "github.com/superchalupa/go-redfish/src/dell-ec"
)

func main() {
	flag.StringSliceP("listen", "l", []string{}, "Listen address.  Formats: (http:[ip]:nn, fcgi:[ip]:port, fcgi:/path, https:[ip]:port, spacemonkey:[ip]:port)")

	var cfgMgrMu sync.Mutex
	cfgMgr := viper.New()
	if err := cfgMgr.BindPFlags(flag.CommandLine); err != nil {
		fmt.Fprintf(os.Stderr, "Could not bind viper flags: %s\n", err)
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
	}

	// Defaults
	cfgMgr.SetDefault("listen", []string{"https::8443"})
	cfgMgr.SetDefault("session.timeout", 10)

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)

	logger := initializeApplicationLogging(cfgMgr)

	domainObjs, _ := domain.NewDomainObjects()
	domainObjs.EventPublisher.AddObserver(logger)
	domainObjs.CommandHandler = logger.makeLoggingCmdHandler(domainObjs.CommandHandler)

	// This also initializes all of the plugins
	domain.InitDomain(ctx, domainObjs.CommandHandler, domainObjs.EventBus, domainObjs.EventWaiter)

	// These three all set up a waiter for the root service to appear, so init root service after.
	stdcollections.InitService(ctx, domainObjs.CommandHandler, domainObjs.EventBus, domainObjs.EventWaiter)
	actionhandler.InitService(ctx, domainObjs.CommandHandler, domainObjs.EventBus, domainObjs.EventWaiter)

	idrac_mvc := idrac.New(ctx, logger, cfgMgr, &cfgMgrMu, domainObjs.CommandHandler, domainObjs.EventBus, domainObjs.EventWaiter)

	cfgMgr.OnConfigChange(func(e fsnotify.Event) {
		cfgMgrMu.Lock()
		defer cfgMgrMu.Unlock()
		logger.Info("CONFIG file changed", "config_file", e.Name)
		for _, fn := range logger.ConfigChangeHooks {
			fn()
		}
		idrac_mvc.ConfigChangeHandler()
	})
	cfgMgr.WatchConfig()

	// Handle the API.
	m := mux.NewRouter()
	loggingHTTPHandler := logger.makeLoggingHTTPHandler(m)

	// per spec: redirect /redfish to /redfish/
	m.Path("/redfish").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/redfish/", 301) })
	// per spec: hardcoded output for /redfish/ to list versions supported.
	m.Path("/redfish/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("{\n\t\"v1\": \"/redfish/v1/\"\n}\n")) })
	// per spec: redirect /redfish/v1 to /redfish/v1/
	m.Path("/redfish/v1/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/redfish/v1", 301) })

	// some static files that we should generate at some point
	m.Path("/redfish/v1/$metadata").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "v1/metadata.xml") })
	m.Path("/redfish/v1/odata").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "v1/odata.json") })

	// serve up the schema XML
	m.PathPrefix("/schemas/v1/").Handler(http.StripPrefix("/schemas/v1/", http.FileServer(http.Dir("./v1/schemas/"))))

	// generic handler for redfish output on most http verbs
	// Note: this works by using the session service to get user details from token to pass up the stack using the embedded struct
	chainAuth := func(u string, p []string) http.Handler { return domain.NewRedfishHandler(domainObjs, logger, u, p) }
	m.PathPrefix("/redfish/v1").Methods("GET", "PUT", "POST", "PATCH", "DELETE", "HEAD", "OPTIONS").HandlerFunc(
		idrac_mvc.GetSessionSvc().MakeHandlerFunc(domainObjs.EventBus, domainObjs, chainAuth, idrac_mvc.GetBasicAuthSvc().MakeHandlerFunc(chainAuth, chainAuth("UNKNOWN", []string{"Unauthenticated"}))))

	// SSE
	chainAuthSSE := func(u string, p []string) http.Handler { return domain.NewSSEHandler(domainObjs, logger, u, p) }
	m.PathPrefix("/events").Methods("GET").HandlerFunc(
		idrac_mvc.GetSessionSvc().MakeHandlerFunc(domainObjs.EventBus, domainObjs, chainAuthSSE, idrac_mvc.GetBasicAuthSvc().MakeHandlerFunc(chainAuthSSE, chainAuth("UNKNOWN", []string{"Unauthenticated"}))))

	// backend command handling
	m.PathPrefix("/api/{command}").Handler(domainObjs.GetInternalCommandHandler(ctx))

	tlscfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// TODO: cli option to enable/disable
		// Secure, but way too slow
		// CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},

		// TODO: cli option to enable/disable
		// Secure, but way too slow
		// PreferServerCipherSuites: true,

		// TODO: cli option to enable/disable
		// Can't quite remember, but I think this breaks curl
		/*		CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			}, */
	}

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
			tlscert.GenRSA(4096),
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
			tlscert.GenRSA(4096),
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

	if len(cfgMgr.GetStringSlice("listen")) == 0 {
		fmt.Fprintf(os.Stderr, "No listeners configured! Use the '-l' option to configure a listener!")
	}

	// And finally, start up all of the listeners that we have configured
	for _, listen := range cfgMgr.GetStringSlice("listen") {
		switch {
		case strings.HasPrefix(listen, "pprof:"):
			pprofMux := http.DefaultServeMux
			http.DefaultServeMux = http.NewServeMux()
			go func(listen string) {
				addr := strings.TrimPrefix(listen, "pprof:")
				s := &http.Server{
					Addr:    addr,
					Handler: pprofMux,
				}
				connectToContext(ctx, logger, s) // make sure when background context is cancelled, this server shuts down cleanly
				logger.Info("PPROF activated with tcp listener: " + addr)
				s.ListenAndServe()
			}(listen)

		case strings.HasPrefix(listen, "fcgi:") && strings.Contains(strings.TrimPrefix(listen, "fcgi:"), ":"):
			// FCGI listener on a TCP socket (usually should be specified as 127.0.0.1 for security)  fcgi:127.0.0.1:4040
			go func(addr string) {
				listener, err := net.Listen("tcp", addr)
				if err != nil {
					logger.Crit("fatal", "Could not open listening connection", "err", err)
					return
				}
				defer listener.Close()
				logger.Info("FCGI mode activated with tcp listener: " + addr)
				logger.Info("Server exited", "err", fcgi.Serve(listener, loggingHTTPHandler))
			}(strings.TrimPrefix(listen, "fcgi:"))

		case strings.HasPrefix(listen, "fcgi:") && strings.Contains(strings.TrimPrefix(listen, "fcgi:"), "/"):
			// FCGI listener on unix domain socket, specified as a path fcgi:/run/fcgi.sock
			go func(path string) {
				listener, err := net.Listen("unix", path)
				if err != nil {
					logger.Crit("fatal", "Could not open listening connection", "err", err)
					return
				}
				defer listener.Close()
				defer os.Remove(path)
				logger.Info("FCGI mode activated with unix socket listener: " + path)
				logger.Info("Server exited", "err", fcgi.Serve(listener, loggingHTTPHandler))
			}(strings.TrimPrefix(listen, "fcgi:"))

		case strings.HasPrefix(listen, "fcgi:"):
			// FCGI listener using stdin/stdout  fcgi:
			go func() {
				logger.Info("FCGI mode activated with stdin/stdout listener")
				logger.Info("Server exited", "err", fcgi.Serve(nil, loggingHTTPHandler))
			}()

		case strings.HasPrefix(listen, "http:"):
			// HTTP protocol listener
			// "https:[addr]:port
			go func(addr string) {
				logger.Info("HTTP listener starting on " + addr)
				s := &http.Server{
					Addr:           addr,
					Handler:        loggingHTTPHandler,
					MaxHeaderBytes: 1 << 20,
					ReadTimeout:    10 * time.Second,
					// cannot use writetimeout if we are streaming
					// WriteTimeout:   10 * time.Second,
				}
				connectToContext(ctx, logger, s) // make sure when background context is cancelled, this server shuts down cleanly
				logger.Info("Server exited", "err", s.ListenAndServe())
			}(strings.TrimPrefix(listen, "http:"))

		case strings.HasPrefix(listen, "https:"):
			// HTTPS protocol listener
			// "https:[addr]:port,certfile,keyfile
			go func(addr string) {
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
				connectToContext(ctx, logger, s) // make sure when background context is cancelled, this server shuts down cleanly
				logger.Info("HTTPS listener starting on " + addr)
				logger.Info("Server exited", "err", s.ListenAndServeTLS("server.crt", "server.key"))
			}(strings.TrimPrefix(listen, "https:"))

		case strings.HasPrefix(listen, "spacemonkey:"):
			// openssl based https
			go runSpaceMonkey(strings.TrimPrefix(listen, "spacemonkey:"), loggingHTTPHandler)
		}
	}

	logger.Debug("Listening", "module", "main", "addresses", fmt.Sprintf("%v\n", cfgMgr.GetStringSlice("listen")))

	// wait until we get an interrupt (CTRL-C)
	<-intr
	cancel()
	logger.Warn("Bye!", "module", "main")
}

type shutdowner interface {
	Shutdown(context.Context) error
}

func connectToContext(ctx context.Context, logger log.Logger, srv interface{}) {
	if s, ok := srv.(shutdowner); ok {
		logger.Info("Hooking up shutdown context.")
		if err := s.Shutdown(ctx); err != nil {
			logger.Info("server_error", err)
		}
	} else {
		logger.Info("Can't cleanly shutdown listener, it will ungracefully end")
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
