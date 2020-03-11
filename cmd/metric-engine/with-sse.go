// +build sse

package main

import (
	"fmt"
	"os"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/cmd/metric-engine/httpcommon"
	"github.com/superchalupa/sailfish/src/http_sse"
	log "github.com/superchalupa/sailfish/src/log"
)

// nolint: gochecknoinits
// have to have init() function to runtime register the compile-time optional components, dont see any other clean way to do this
func init() {
	initOptional()
	optionalComponents = append([]func(log.Logger, *viper.Viper, busIntf) func(){
		func(logger log.Logger, cfg *viper.Viper, d busIntf) func() {
			serverlist := createHTTPServerBookkeeper(logger)
			addSSEHandlers(logger, cfg, d, serverlist)
			return nil
		}}, optionalComponents...)
}

func addSSEHandlers(logger log.Logger, cfgMgr *viper.Viper, d busIntf, serverlist *httpcommon.ServerTracker) {
	logger.Crit("SSE ENABLED")
	cfgMgr.SetDefault("sse", "unix:/run/telemetryservice/http.socket")

	listenAddrs := cfgMgr.GetStringSlice("sse")
	if len(listenAddrs) == 0 {
		fmt.Fprintf(os.Stderr, "No SSE listeners configured! Use the 'sse' YAML option to configure a listener!")
		return
	}

	// hint to the runtime it can release this memory
	cfgMgr = nil

	for _, listen := range listenAddrs {
		m := serverlist.GetHandler(listen)

		m.Path("/events").
			Methods("GET").
			Handler(http_sse.NewSSEHandler(d, logger, "UNKNOWN", []string{"Unauthenticated"}, func(eh.Event) bool { return true }))
	}
}
