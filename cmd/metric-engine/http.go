// +build pprof redfish sse

package main

import (
	"sync"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/httpcommon"
	log "github.com/superchalupa/sailfish/src/log"
)

// nolint: gochecknoglobals
// couldnt really find a better way of doing the compile time optional registration, so basically need some globals
var (
	reglock    = sync.Once{}
	runlock    = sync.Once{}
	serverlist *httpcommon.ServerTracker
)

// nolint: gochecknoinits
// have to have init() function to runtime register the compile-time optional components, dont see any other clean way to do this
func init() {
	initOptional()
	// start the http servers after we've attached all handlers. gorilla mux has limitation that you must not add routers after server startup
	optionalComponents = append(optionalComponents, func(logger log.Logger, cfg *viper.Viper, d busIntf) func() {
		serverlist := createHTTPServerBookkeeper(logger)
		runservers(logger)
		return func() { serverlist.Shutdown() }
	})
}

func createHTTPServerBookkeeper(logger log.Logger) *httpcommon.ServerTracker {
	reglock.Do(func() {
		serverlist = httpcommon.New(logger)
	})
	return serverlist
}

func runservers(logger log.Logger) {
	runlock.Do(func() {
		serverlist.ListenAndServe(logger)
	})
}
