package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/gorilla/mux"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	// space monkey (openssl wrapper for go)
	"github.com/spacemonkeygo/openssl"

	// auth plugins
	"github.com/superchalupa/go-redfish/plugins/basicauth"
	"github.com/superchalupa/go-redfish/plugins/session"

	// cert gen
	"github.com/superchalupa/go-redfish/plugins/tlscert"

	// load plugins (auto-register)
	_ "github.com/superchalupa/go-redfish/plugins/actionhandler"
	_ "github.com/superchalupa/go-redfish/plugins/patch"
	_ "github.com/superchalupa/go-redfish/plugins/rootservice"
	_ "github.com/superchalupa/go-redfish/plugins/runcmd"
	_ "github.com/superchalupa/go-redfish/plugins/stdcollections"

	// Test plugins
	_ "github.com/superchalupa/go-redfish/plugins/test"
	_ "github.com/superchalupa/go-redfish/plugins/test_action"
)

// Define a type named "strslice" as a slice of strings
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

func main() {
	log.Println("starting backend")
	var (
		configFile  = flag.String("config", "app.yaml", "Application configuration file")
		listenAddrs strslice
	)
	flag.Var(&listenAddrs, "l", "Listen address.  Formats: (:nn, fcgi:ip:port, fcgi:/path)")
	flag.Parse()

	cfg, _ := loadConfig(*configFile)
	if len(listenAddrs) > 0 {
		cfg.Listen = listenAddrs
	}

	ctx, cancel := context.WithCancel(context.Background())
	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)

	domainObjs, _ := domain.NewDomainObjects()
	domainObjs.EventPublisher.AddObserver(&Logger{})
	domainObjs.CommandHandler = makeLoggingCmdHandler(domainObjs.CommandHandler)

	// This also initializes all of the plugins
	domain.InitDomain(ctx, domainObjs.CommandHandler, domainObjs.EventBus, domainObjs.EventWaiter)

	// Set up our standard extensions for authentication
	// the authentication plugin will explicitly pass username to the final handler using the chainAuth() function
	// the authorization plugin will explicitly pass the privileges to the final handler using the chainAuth function
	chainAuth := func(u string, p []string) http.Handler {
		return domain.NewRedfishHandler(domainObjs, u, p)
	}
	BasicAuthAuthorizer := basicauth.NewService()
	sessionServiceAuthorizer := session.NewService(domainObjs.EventBus, domainObjs)
	sessionServiceAuthorizer.OnUserDetails = chainAuth
	sessionServiceAuthorizer.WithoutUserDetails = BasicAuthAuthorizer
	BasicAuthAuthorizer.OnUserDetails = chainAuth
	BasicAuthAuthorizer.WithoutUserDetails = domain.NewRedfishHandler(domainObjs, "UNKNOWN", []string{"Unauthenticated"})

	// same thing for SSE
	chainAuthSSE := func(u string, p []string) http.Handler {
		return domain.NewSSEHandler(domainObjs, u, p)
	}
	BasicAuthAuthorizerSSE := basicauth.NewService()
	sessionServiceAuthorizerSSE := session.NewService(domainObjs.EventBus, domainObjs)
	sessionServiceAuthorizerSSE.OnUserDetails = chainAuthSSE
	sessionServiceAuthorizerSSE.WithoutUserDetails = BasicAuthAuthorizerSSE
	BasicAuthAuthorizerSSE.OnUserDetails = chainAuthSSE
	BasicAuthAuthorizerSSE.WithoutUserDetails = domain.NewSSEHandler(domainObjs, "UNKNOWN", []string{"Unauthenticated"})

	// Handle the API.
	m := mux.NewRouter()

	// per spec: redirect /redfish to /redfish/
	m.Path("/redfish").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/redfish/", 301) })
	// per spec: hardcoded output for /redfish/ to list versions supported.
	m.Path("/redfish/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("{\n\t\"v1\": \"/redfish/v1/\"\n}\n")) })
	// per spec: redirect /redfish/v1 to /redfish/v1/
	m.Path("/redfish/v1/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/redfish/v1", 301) })

	// some static files that we should generate at some point
	m.Path("/redfish/v1/$metadata").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "v1/metadata.xml") })
	m.Path("/redfish/v1/odata").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "v1/odata.json") })

	// generic handler for redfish output on most http verbs
	// Note: this works by using the session service to get user details from token to pass up the stack using the embedded struct
	m.PathPrefix("/redfish/v1").Methods("GET", "PUT", "POST", "PATCH", "DELETE", "HEAD", "OPTIONS").Handler(sessionServiceAuthorizer)

	// SSE
	m.PathPrefix("/events").Methods("GET").Handler(sessionServiceAuthorizerSSE)

	// backend command handling
	m.PathPrefix("/api/{command}").Handler(domainObjs.GetInternalCommandHandler())

	// profiling stuff
	// TODO: cli option to enable/disable
	m.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	m.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	m.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	m.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	m.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	// Simple HTTP request logging.
	logger := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(begin time.Time) {
			log.Println(
				"source", r.RemoteAddr,
				"method", r.Method,
				"url", r.URL,
				"business_logic_time", time.Since(begin),
				"args", fmt.Sprintf("%#v", mux.Vars(r)),
			)
		}(time.Now())
		m.ServeHTTP(w, r)
	})

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
	// Create CA cert if it doesn't exist, or load one if it does
	ca, _ := tlscert.NewCert(
		tlscert.CreateCA,
		tlscert.ExpireInOneYear,
		tlscert.SetCommonName("CA Cert common name"),
		tlscert.SetSerialNumber(12345),
		tlscert.SetBaseFilename("ca"),
		tlscert.LoadIfExists(), // this should be last
	)
	ca.Serialize()

	// TODO: cli option to enable/disable and control cert options
	// create new server cert unconditionally based on CA cert
	var Options []tlscert.Option
	Options = append(Options, tlscert.SignWithCA(ca))
	Options = append(Options, tlscert.MakeServer)
	Options = append(Options, tlscert.ExpireInOneYear)
	Options = append(Options, tlscert.SetCommonName("localhost"))
	Options = append(Options, tlscert.SetSubjectKeyId([]byte{1, 2, 3, 4, 6}))
	Options = append(Options, tlscert.AddSANDNSName("localhost", "localhost.localdomain"))
	Options = append(Options, tlscert.SetSerialNumber(12346))
	Options = append(Options, tlscert.SetBaseFilename("server"))
	iterInterfaceIPAddrs(func(ip net.IP) { Options = append(Options, tlscert.AddSANIP(ip)) })
	serverCert, _ := tlscert.NewCert(Options...)
	serverCert.Serialize()

	servers := []*http.Server{}
	if len(cfg.Listen) == 0 {
		log.Fatal("No listeners configured! Use the '-l' option to configure a listener!")
	}

	// And finally, start up all of the listeners that we have configured
	for _, listen := range cfg.Listen {
		switch {
		case strings.HasPrefix(listen, "fcgi:") && strings.Contains(strings.TrimPrefix(listen, "fcgi:"), ":"):
			// FCGI listener on a TCP socket (usually should be specified as 127.0.0.1 for security)  fcgi:127.0.0.1:4040
			go func(addr string) {
				listener, err := net.Listen("tcp", addr)
				if err != nil {
					log.Println("fatal", "Could not open listening connection", "err", err)
					return
				}
				defer listener.Close()
				log.Println("FCGI mode activated with tcp listener: " + addr)
				log.Println(fcgi.Serve(listener, m))
			}(strings.TrimPrefix(listen, "fcgi:"))

		case strings.HasPrefix(listen, "fcgi:") && strings.Contains(strings.TrimPrefix(listen, "fcgi:"), "/"):
			// FCGI listener on unix domain socket, specified as a path fcgi:/run/fcgi.sock
			go func(path string) {
				listener, err := net.Listen("unix", path)
				if err != nil {
					log.Println("fatal", "Could not open listening connection", "err", err)
					return
				}
				defer listener.Close()
				defer os.Remove(path)
				log.Println("FCGI mode activated with unix socket listener: " + path)
				log.Println(fcgi.Serve(listener, m))
			}(strings.TrimPrefix(listen, "fcgi:"))

		case strings.HasPrefix(listen, "fcgi:"):
			// FCGI listener using stdin/stdout  fcgi:
			go func() {
				log.Println("FCGI mode activated with stdin/stdout listener")
				log.Println(fcgi.Serve(nil, m))
			}()

		case strings.HasPrefix(listen, "http:"):
			// HTTP protocol listener
			// "https:[addr]:port
			go func(addr string) {
				log.Println("HTTP listener starting on " + addr)
				s := &http.Server{
					Addr:           addr,
					Handler:        logger,
					MaxHeaderBytes: 1 << 20,
					ReadTimeout:    10 * time.Second,
					// cannot use writetimeout if we are streaming
					// WriteTimeout:   10 * time.Second,
				}
				servers = append(servers, s)
				log.Println(s.ListenAndServe())
			}(strings.TrimPrefix(listen, "http:"))

		case strings.HasPrefix(listen, "https:"):
			// HTTPS protocol listener
			// "https:[addr]:port,certfile,keyfile
			go func(addr string) {
				s := &http.Server{
					Addr:           addr,
					Handler:        logger,
					MaxHeaderBytes: 1 << 20,
					ReadTimeout:    10 * time.Second,
					// cannot use writetimeout if we are streaming
					// WriteTimeout:   10 * time.Second,
					TLSConfig: tlscfg,
					// can't remember why this doesn't work... TODO: reason this doesnt work
					//TLSNextProto:   make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
				}
				servers = append(servers, s)
				log.Println("HTTPS listener starting on " + addr)
				log.Println(s.ListenAndServeTLS("server.crt", "server.key"))
			}(strings.TrimPrefix(listen, "https:"))

		case strings.HasPrefix(listen, "spacemonkey:"):
			// HTTPS protocol listener
			// "https:[addr]:port,certfile,keyfile
			go func(addr string) {
				log.Println("OPENSSL(spacemonkey) listener starting")
				log.Fatal(openssl.ListenAndServeTLS(addr, "server.crt", "server.key", logger))
			}(strings.TrimPrefix(listen, "spacemonkey:"))
		}
	}

	fmt.Printf("%v\n", listenAddrs)

	// wait until we get an interrupt (CTRL-C)
	<-intr
	cancel()
	fmt.Printf("\ninterrupted\n")

	type Shutdowner interface {
		Shutdown(context.Context) error
	}

	for _, srv := range servers {
		// go 1.7 doesn't have Shutdown method on http server, so optionally cast
		// the interface to see if it exists, then call it, if possible Can
		// only do this with interfaces not concrete structs, so define a func
		// taking needed interface and call it.
		func(srv interface{}, addr string) {
			if s, ok := srv.(Shutdowner); ok {
				log.Println("shutting down listener: " + addr)
				if err := s.Shutdown(nil); err != nil {
					log.Println("server_error", err)
				}
			} else {
				log.Println("Can't cleanly shutdown listener, it will ungracefully end: " + addr)
			}
		}(srv, srv.Addr)
	}

	fmt.Printf("Bye!\n")
}

// Create a tiny logging middleware for the command handler.
func makeLoggingCmdHandler(originalHandler eh.CommandHandler) eh.CommandHandler {
	return eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		log.Printf("CMD %#v", cmd)
		return originalHandler.HandleCommand(ctx, cmd)
	})
}

// Logger is a simple event handler for logging all events.
type Logger struct{}

// Notify implements the Notify method of the EventObserver interface.
func (l *Logger) Notify(ctx context.Context, event eh.Event) {
	log.Printf("EVENT %s", event)
}

func iterInterfaceIPAddrs(fn func(net.IP)) {
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
			fmt.Printf("Adding local IP Address to server cert as SAN: %s\n", ip)
			fn(ip)
		}
	}
}
