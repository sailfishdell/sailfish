// +build sailfisheventservice

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/httpcommon"
	"github.com/superchalupa/sailfish/src/http_redfish_sse"
	log "github.com/superchalupa/sailfish/src/log"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	"github.com/superchalupa/sailfish/src/dell-msm"

	"github.com/superchalupa/sailfish/src/dell-resources/dellauth"
	"github.com/superchalupa/sailfish/src/ocp/basicauth"
	"github.com/superchalupa/sailfish/src/ocp/session"
)

// nolint: gochecknoinits
// have to have init() function to runtime register the compile-time optional components, dont see any other clean way to do this
func init() {
	initOptional()
	optionalComponents = append([]func(log.Logger, *viper.Viper, busIntf) func(){
		func(logger log.Logger, cfg *viper.Viper, d busIntf) func() {
			serverlist := createHTTPServerBookkeeper(logger)
			err := addSailfishHandlers(logger, cfg, d, serverlist)
			if err != nil {
				panic("sailfish startup init failed: " + err.Error())
			}
			return nil
		}}, optionalComponents...)
}

func addSailfishHandlers(logger log.Logger, cfgMgr *viper.Viper, d busIntf, serverlist *httpcommon.ServerTracker) error {
	logger.Crit("SAILFISH ENABLED")
	cfgMgr.SetDefault("sailfish", "unix:/run/telemetryservice/http.socket")

	listenAddrs := cfgMgr.GetStringSlice("sailfish")
	if len(listenAddrs) == 0 {
		fmt.Fprintf(os.Stderr, "No SAILFISH listeners configured! Use the 'sailfish' YAML option to configure a listener!")
		return nil
	}

	ctx := context.Background()

	// todo: add option functions to pass in existing bus stuff
	domainObjs, _ := domain.NewDomainObjects(
		domain.WithLogger(logger),
		domain.WithBus(d.GetBus()),
		domain.WithPublisher(d.GetPublisher()),
		domain.WithWaiter(d.GetWaiter()))
	domain.InitDomain(ctx, domainObjs.CommandHandler, domainObjs.EventBus, domainObjs.EventWaiter)

	// generic handler for redfish output on most http verbs
	// Note: this works by using the session service to get user details from token to pass up the stack using the embedded struct
	chainAuth := func(u string, p []string) http.Handler { return domain.NewRedfishHandler(domainObjs, logger, u, p) }

	handlerFunc := dellauth.MakeHandlerFunc(chainAuth,
		session.MakeHandlerFunc(logger, domainObjs.EventBus, domainObjs, chainAuth,
			basicauth.MakeHandlerFunc(chainAuth,
				chainAuth("UNKNOWN", []string{"Unauthenticated"}))))

	// Redfish SSE
	chainAuthRFSSE := func(u string, p []string) http.Handler {
		return http_redfish_sse.NewRedfishSSEHandler(domainObjs, logger, u, p)
	}

	for _, listen := range listenAddrs {
		m := serverlist.GetHandler(listen)
		logger.Crit("Adding route")

		m.Path("/redfish/v1/EventService/SSE").Methods("GET").HandlerFunc(
			session.MakeHandlerFunc(logger, domainObjs.EventBus, domainObjs, chainAuthRFSSE, basicauth.MakeHandlerFunc(chainAuthRFSSE, chainAuthRFSSE("UNKNOWN", []string{"Unauthenticated"}))))

		// All of the /redfish apis
		m.PathPrefix("/redfish/v1/EventService").Methods("GET", "PUT", "POST", "PATCH", "DELETE", "HEAD", "OPTIONS").HandlerFunc(handlerFunc)
	}

	configRWLock := &sync.RWMutex{}
	dell_msm.New(ctx, logger, cfgMgr, configRWLock, domainObjs)

	return nil
}
