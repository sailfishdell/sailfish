package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"

	domain "github.com/superchalupa/go-redfish/redfishresource"

	// auth plugins
	"github.com/superchalupa/go-redfish/plugins/basicauth"
	"github.com/superchalupa/go-redfish/plugins/session"

	// cert gen
	"github.com/superchalupa/go-redfish/plugins/tlscert"

	// load plugins (auto-register)
	_ "github.com/superchalupa/go-redfish/plugins/actionhandler"
	_ "github.com/superchalupa/go-redfish/plugins/runcmd"
	_ "github.com/superchalupa/go-redfish/plugins/stdcollections"
	_ "github.com/superchalupa/go-redfish/plugins/patch"
	// Test plugins
	_ "github.com/superchalupa/go-redfish/plugins/test"
	_ "github.com/superchalupa/go-redfish/plugins/test_action"
)

func main() {
	log.Println("starting backend")

	domainObjs, _ := domain.NewDomainObjects()
	domainObjs.EventPublisher.AddObserver(&Logger{})

	orighandler := domainObjs.CommandHandler

	// Create a tiny logging middleware for the command handler.
	loggingHandler := eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		log.Printf("CMD %#v", cmd)
		return orighandler.HandleCommand(ctx, cmd)
	})

	domainObjs.CommandHandler = loggingHandler

	// This also initializes all of the plugins
	domain.InitDomain(context.Background(), domainObjs.CommandHandler, domainObjs.EventBus, domainObjs.EventWaiter)

	// generate uuid of root object
	rootID := eh.NewUUID()

	// Set up our standard extensions for authentication
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

	// set up some basic stuff
	loggingHandler.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          rootID,
			ResourceURI: "/redfish/v1",
			Type:        "#ServiceRoot.v1_0_2.ServiceRoot",
			Context:     "/redfish/v1/$metadata#ServiceRoot.ServiceRoot",
			// anybody can access
			Privileges: map[string]interface{}{"GET": []string{"Unauthenticated"}},
			Properties: map[string]interface{}{
				"Id":             "RootService",
				"Name":           "Root Service",
				"RedfishVersion": "1.0.2",
				"UUID":           "92384634-2938-2342-8820-489239905423",
			},
		},
	)

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
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
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

	// Create CA cert, or load
	ca, _ := tlscert.NewCert(
		tlscert.CreateCA,
		tlscert.ExpireInOneYear,
		tlscert.SetCommonName("CA Cert common name"),
		tlscert.SetSerialNumber(12345),
		tlscert.SetBaseFilename("ca"),
		tlscert.LoadIfExists(), // this should be last
	)
	ca.Serialize()

	var Options []tlscert.Option
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
			Options = append(Options, tlscert.AddSANIP(ip))
		}
	}

	Options = append(Options, tlscert.SignWithCA(ca))
	Options = append(Options, tlscert.MakeServer)
	Options = append(Options, tlscert.ExpireInOneYear)
	Options = append(Options, tlscert.SetCommonName("localhost"))
	Options = append(Options, tlscert.SetSubjectKeyId([]byte{1, 2, 3, 4, 6}))
	Options = append(Options, tlscert.AddSANDNSName("localhost", "localhost.localdomain"))
	Options = append(Options, tlscert.AddSANIPAddress("127.0.0.1"))
	Options = append(Options, tlscert.SetSerialNumber(12346))
	Options = append(Options, tlscert.SetBaseFilename("server"))
	//Options = append(Options, tlscert.LoadIfExists()) // this should be last

	// create new server cert or load from disk
	serverCert, _ := tlscert.NewCert(Options...)
	serverCert.Serialize()

	s := &http.Server{
		Addr:        ":8443",
		Handler:     logger,
		ReadTimeout: 10 * time.Second,
		// cannot use writetimeout if we are streaming
		// WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      tlscfg,
		//TLSNextProto:   make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}

	log.Println(s.ListenAndServeTLS("server.crt", "server.key"))
}

// Logger is a simple event handler for logging all events.
type Logger struct{}

// Notify implements the Notify method of the EventObserver interface.
func (l *Logger) Notify(ctx context.Context, event eh.Event) {
	log.Printf("EVENT %s", event)
}
