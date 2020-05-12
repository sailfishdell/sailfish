package httpcommon

import (
	"context"
	"crypto/elliptic"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/tlscert"
)

const (
	MaxHeaderBytes        = 1 << 20
	ReadTimeout           = 100 * time.Second
	DefaultCASerialNumber = 12345
	DefaultSerialNumber   = 12346
)

type shutdowner interface {
	Shutdown(context.Context) error
}

type ServerTracker struct {
	logger       log.Logger
	handlers     map[string]*mux.Router
	servers      []func()
	shutdownlist []shutdowner
}

func New(logger log.Logger) *ServerTracker {
	return &ServerTracker{
		logger:       logger,
		handlers:     map[string]*mux.Router{},
		servers:      []func(){},
		shutdownlist: []shutdowner{},
	}
}

func (st *ServerTracker) GetHandler(name string) *mux.Router {
	h, ok := st.handlers[name]
	if !ok {
		h = mux.NewRouter()
		st.handlers[name] = h
		st.startServer(h, name)
	}
	return h
}

func (st *ServerTracker) ListenAndServe(logger log.Logger) {
	for _, fn := range st.servers {
		go fn()
	}
}

func (st *ServerTracker) addShutdown(name string, srv interface{}) {
	if srv == nil {
		return
	}
	s, ok := srv.(shutdowner)
	if !ok {
		st.logger.Crit("The requested HTTP server can't be gracefully shutdown at program exit.", "ServerName", name)
		return
	}
	st.shutdownlist = append(st.shutdownlist, s)
}

func (st *ServerTracker) Shutdown() {
	// wait up to 1 second for active connections
	// SSE bus always "hangs" because there is always an active connection
	shutdownCtx, cancelshutdown := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelshutdown()
	for _, s := range st.shutdownlist {
		err := s.Shutdown(shutdownCtx)
		if err != nil {
			st.logger.Crit("server wasn't gracefully shutdown.", "err", err)
		}
	}

	st.shutdownlist = []shutdowner{}

	st.logger.Warn("Bye!", "module", "main")
}

func (st *ServerTracker) startServer(handler http.Handler, listen string) {
	switch {
	case strings.HasPrefix(listen, "http:"):
		// HTTP protocol listener
		// "https:[addr]:port
		addr := strings.TrimPrefix(listen, "http:")
		st.logger.Crit("HTTP listener starting on " + addr)
		s := &http.Server{
			Addr:           addr,
			Handler:        handlers.LoggingHandler(os.Stderr, handler),
			MaxHeaderBytes: MaxHeaderBytes,
			ReadTimeout:    ReadTimeout,
			// cannot use writetimeout if we are streaming
			// WriteTimeout:   10 * time.Second,
		}
		st.servers = append(st.servers, func() { st.logger.Info("Server exited", "err", s.ListenAndServe()) })
		st.addShutdown(listen, s)

	case strings.HasPrefix(listen, "unix:"):
		// HTTP protocol listener
		// "https:[addr]:port
		addr := strings.TrimPrefix(listen, "unix:")
		st.logger.Crit("UNIX SOCKET listener starting on " + addr)
		os.Remove(addr)
		s := &http.Server{
			Handler:        handlers.LoggingHandler(os.Stderr, handler),
			MaxHeaderBytes: MaxHeaderBytes,
			ReadTimeout:    ReadTimeout,
		}
		unixListener, err := net.Listen("unix", addr)
		if err != nil {
			panic("Could not start required listener(" + addr + "). The error was: " + err.Error())
		}
		st.servers = append(st.servers, func() { st.logger.Info("Server exited", "err", s.Serve(unixListener)) })
		st.addShutdown(listen, s)

	case strings.HasPrefix(listen, "https:"):
		// HTTPS protocol listener
		// "https:[addr]:port,certfile,keyfile
		addr := strings.TrimPrefix(listen, "https:")
		s := &http.Server{
			Addr:           addr,
			Handler:        handler,
			MaxHeaderBytes: MaxHeaderBytes,
			ReadTimeout:    ReadTimeout,
			// cannot use writetimeout if we are streaming
			// WriteTimeout:   10 * time.Second,
			TLSConfig: getTLSCfg(),
			// can't remember why this doesn't work... TODO: reason this doesnt work
			//TLSNextProto:   make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		}
		st.logger.Crit("HTTPS listener starting on " + addr)
		checkCaCerts(st.logger)
		st.servers = append(st.servers, func() { st.logger.Info("Server exited", "err", s.ListenAndServeTLS("server.crt", "server.key")) })
		st.addShutdown(listen, s)
	}
}

func getTLSCfg() *tls.Config {
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
			tlscert.SetSerialNumber(DefaultCASerialNumber),
			tlscert.SetBaseFilename("ca"),
			tlscert.GenECDSA(elliptic.P256()),
			tlscert.SelfSigned(),
			tlscert.WithLogger(logger),
		)
		err := ca.Serialize()
		if err != nil {
			logger.Crit("Error serializing cert", "err", err)
		}
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
			tlscert.SetSerialNumber(DefaultSerialNumber),
			tlscert.SetBaseFilename("server"),
			tlscert.WithLogger(logger),
		)
		iterInterfaceIPAddrs(logger, func(ip net.IP) {
			err := serverCert.ApplyOption(tlscert.AddSANIP(ip))
			if err != nil {
				logger.Crit("Error applying local IP to cert", "err", err)
			}
		})
		err := serverCert.Serialize()
		if err != nil {
			logger.Crit("Error serializing cert", "err", err)
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
