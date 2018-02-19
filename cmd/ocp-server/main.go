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
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/gorilla/mux"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	// auth plugins
	"github.com/superchalupa/go-redfish/plugins/basicauth"
	"github.com/superchalupa/go-redfish/plugins/session"

	// cert gen
	"github.com/superchalupa/go-redfish/plugins/tlscert"

	// load plugins (auto-register)
	_ "github.com/superchalupa/go-redfish/plugins/actionhandler"
	_ "github.com/superchalupa/go-redfish/plugins/rootservice"
	_ "github.com/superchalupa/go-redfish/plugins/stdcollections"
	_ "github.com/superchalupa/go-redfish/plugins/stdmeta"

	// load openbmc plugins
	_ "github.com/superchalupa/go-redfish/plugins/obmc"

	// Test plugins (Take these out for a real server)
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
	flag.Var(&listenAddrs, "l", "Listen address.  Formats: (http:[ip]:nn, fcgi:[ip]:port, fcgi:/path, https:[ip]:port, spacemonkey:[ip]:port)")
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

	// serve up the schema XML
	m.PathPrefix("/schemas/v1/").Handler(http.StripPrefix("/schemas/v1/", http.FileServer(http.Dir("./v1/schemas/"))))

	// generic handler for redfish output on most http verbs
	// Note: this works by using the session service to get user details from token to pass up the stack using the embedded struct
	m.PathPrefix("/redfish/v1").Methods("GET", "PUT", "POST", "PATCH", "DELETE", "HEAD", "OPTIONS").Handler(sessionServiceAuthorizer)

	// SSE
	m.PathPrefix("/events").Methods("GET").Handler(sessionServiceAuthorizerSSE)

	// backend command handling
	m.PathPrefix("/api/{command}").Handler(domainObjs.GetInternalCommandHandler(ctx))

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
	// Load CA cert if it exists. Create CA cert if it doesn't.
	ca, err := tlscert.Load("ca")
	if err != nil {
		ca, _ = tlscert.NewCert(
			tlscert.CreateCA,
			tlscert.ExpireInOneYear,
			tlscert.SetCommonName("CA Cert common name"),
			tlscert.SetSerialNumber(12345),
			tlscert.SetBaseFilename("ca"),
			tlscert.GenRSA(4096),
			tlscert.SelfSigned(),
		)
		ca.Serialize()
	}

	// TODO: cli option to enable/disable and control cert options
	// create new server cert unconditionally based on CA cert
	_, err = tlscert.Load("server")
	if err != nil {
		serverCert, _ := tlscert.NewCert(
			tlscert.GenRSA(4096),
			tlscert.SignWithCA(ca),
			tlscert.MakeServer,
			tlscert.ExpireInOneYear,
			tlscert.SetCommonName("localhost"),
			tlscert.SetSubjectKeyId([]byte{1, 2, 3, 4, 6}),
			tlscert.AddSANDNSName("localhost", "localhost.localdomain"),
			tlscert.SetSerialNumber(12346),
			tlscert.SetBaseFilename("server"),
		)
		iterInterfaceIPAddrs(func(ip net.IP) { serverCert.ApplyOption(tlscert.AddSANIP(ip)) })
		serverCert.Serialize()
	}

	if len(cfg.Listen) == 0 {
		log.Fatal("No listeners configured! Use the '-l' option to configure a listener!")
	}

	// And finally, start up all of the listeners that we have configured
	for _, listen := range cfg.Listen {
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
				ConnectToContext(ctx, s) // make sure when background context is cancelled, this server shuts down cleanly
				log.Println("PPROF activated with tcp listener: " + addr)
				s.ListenAndServe()
			}(listen)

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
				ConnectToContext(ctx, s) // make sure when background context is cancelled, this server shuts down cleanly
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
				ConnectToContext(ctx, s) // make sure when background context is cancelled, this server shuts down cleanly
				log.Println("HTTPS listener starting on " + addr)
				log.Println(s.ListenAndServeTLS("server.crt", "server.key"))
			}(strings.TrimPrefix(listen, "https:"))

		case strings.HasPrefix(listen, "spacemonkey:"):
			// openssl based https
			go run_spacemonkey(strings.TrimPrefix(listen, "spacemonkey:"), logger)
		}
	}

	fmt.Printf("%v\n", listenAddrs)

	// wait until we get an interrupt (CTRL-C)
	<-intr
	cancel()
	fmt.Printf("\ninterrupted\n")
	fmt.Printf("Bye!\n")
}

// Create a tiny logging middleware for the command handler.
func makeLoggingCmdHandler(originalHandler eh.CommandHandler) eh.CommandHandler {
	return eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		log.Printf("CMD %#v", cmd)
		return originalHandler.HandleCommand(ctx, cmd)
	})
}

type Shutdowner interface {
	Shutdown(context.Context) error
}

func ConnectToContext(ctx context.Context, srv interface{}) {
	if s, ok := srv.(Shutdowner); ok {
		log.Println("Hooking up shutdown context.")
		if err := s.Shutdown(ctx); err != nil {
			log.Println("server_error", err)
		}
	} else {
		log.Println("Can't cleanly shutdown listener, it will ungracefully end")
	}
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
