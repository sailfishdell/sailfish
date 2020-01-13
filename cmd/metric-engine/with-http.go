// +build http

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
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/gorilla/mux"

	"github.com/superchalupa/sailfish/src/http_inject"
	"github.com/superchalupa/sailfish/src/http_sse"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/tlscert"
)

type shutdowner interface {
	Shutdown(context.Context) error
}

type service struct {
	shutdownlist []shutdowner
	logger       log.Logger
}

func starthttp(logger log.Logger, cfgMgr *viper.Viper, d *BusComponents) (ret *service) {
	injectSvc := http_inject.New(logger, d)
	injectSvc.Start()

	// Set up HTTP endpoints
	m := mux.NewRouter()
	loggingHTTPHandler := makeLoggingHTTPHandler(logger, m)
	m.Path("/events").Methods("GET").Handler(http_sse.NewSSEHandler(d, logger, "UNKNOWN", []string{"Unauthenticated"}))
	m.Path("/api/Event:Inject").Methods("POST").HandlerFunc(injectSvc.GetHandlerFunc())

	listen_addrs := cfgMgr.GetStringSlice("listen")
	if len(listen_addrs) == 0 {
		fmt.Fprintf(os.Stderr, "No listeners configured! Use the '-l' option to configure a listener!")
	}

	// tell the runtime it can release this memory
	cfgMgr = nil

	fmt.Printf("Starting services: %s\n", listen_addrs)

	ret = &service{
		shutdownlist: []shutdowner{},
		logger:       logger,
	}
	addShutdown := func(name string, srv interface{}) {
		if srv == nil {
			return
		}
		s, ok := srv.(shutdowner)
		if !ok {
			logger.Crit("The requested HTTP server can't be gracefully shutdown at program exit.", "ServerName", name)
			return
		}
		ret.shutdownlist = append(ret.shutdownlist, s)
	}

	// And finally, start up all of the listeners that we have configured
	for _, listen := range listen_addrs {
		switch {
		case strings.HasPrefix(listen, "pprof:"):
			addr := strings.TrimPrefix(listen, "pprof:")
			fn, s := runpprof(logger, addr)
			if s != nil {
				addShutdown(listen, s)
			}
			go fn()

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
			addShutdown(listen, s)

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
				addShutdown(listen, s)
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
				TLSConfig: getTlsCfg(),
				// can't remember why this doesn't work... TODO: reason this doesnt work
				//TLSNextProto:   make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
			}
			logger.Info("HTTPS listener starting on " + addr)
			checkCaCerts(logger)
			go func() { logger.Info("Server exited", "err", s.ListenAndServeTLS("server.crt", "server.key")) }()
			addShutdown(listen, s)

		}
	}

	logger.Debug("Listening", "module", "main", "addresses", fmt.Sprintf("%v\n", listen_addrs))
	injectSvc.Ready()
	return
}

func (svc *service) shutdown() {
	// wait up to 1 second for active connections
	// SSE bus always "hangs" because there is always an active connection
	shutdownCtx, cancelshutdown := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelshutdown()
	for _, s := range svc.shutdownlist {
		err := s.Shutdown(shutdownCtx)
		if err != nil {
			svc.logger.Crit("server wasn't gracefully shutdown.", "err", err)
		}
	}

	svc.logger.Warn("Bye!", "module", "main")
}

func getTlsCfg() *tls.Config {
	return &tls.Config{
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

func makeLoggingHTTPHandler(l log.Logger, m http.Handler) http.HandlerFunc {
	// Simple HTTP request logging.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(begin time.Time) {
			l.Info(
				"Processed http request",
				"source", r.RemoteAddr,
				"method", r.Method,
				"url", r.URL,
				"business_logic_time", time.Since(begin),
				"module", "http",
				"args", fmt.Sprintf("%#v", mux.Vars(r)),
			)
		}(time.Now())
		m.ServeHTTP(w, r)
	})
}
