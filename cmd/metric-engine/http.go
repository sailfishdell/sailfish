// +build pprof redfish sse

package main

import (
	"sync"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/httpcommon"
	log "github.com/superchalupa/sailfish/src/log"
)

var reglock = sync.Once{}
var runlock = sync.Once{}
var serverlist *httpcommon.ServerTracker

func init() {
	initOptional()
	// start the http servers after we've attached all handlers. gorilla mux has limitation that you must not add routers after server startup
	optionalComponents = append(optionalComponents, func(logger log.Logger, cfg *viper.Viper, d *busComponents) func() {
		serverlist := createHttpServerBookkeeper(logger)
		runservers(logger)
		return func() { serverlist.Shutdown() }
	})
}

func createHttpServerBookkeeper(logger log.Logger) *httpcommon.ServerTracker {
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
