// +build pprof

package main

import (
	"fmt"
	"net/http/pprof"
	"os"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/httpcommon"
	log "github.com/superchalupa/sailfish/src/log"
)

// nolint: gochecknoinits
// have to have init() function to runtime register the compile-time optional components, dont see any other clean way to do this
func init() {
	initOptional()
	optionalComponents = append([]func(log.Logger, *viper.Viper, busIntf) func(){
		func(logger log.Logger, cfg *viper.Viper, d busIntf) func() {
			serverlist := createHTTPServerBookkeeper(logger)
			addPprofHandlers(logger, cfg, serverlist)
			return nil
		}}, optionalComponents...)
}

func addPprofHandlers(logger log.Logger, cfgMgr *viper.Viper, serverlist *httpcommon.ServerTracker) {
	logger.Crit("PPROF ENABLED")
	cfgMgr.SetDefault("pprof", "unix:/run/telemetryservice/http.socket")

	listenAddrs := cfgMgr.GetStringSlice("pprof")
	if len(listenAddrs) == 0 {
		fmt.Fprintf(os.Stderr, "No PPROF listeners configured! Use the 'pprof' YAML option to configure a listener!")
		return
	}

	// hint to the runtime it can release this memory
	cfgMgr = nil

	for _, listen := range listenAddrs {
		m := serverlist.GetHandler(listen)
		logger.Crit("Add PPROF routes to handler", "listen", listen, "handler", m)

		m.HandleFunc("/debug/pprof/", pprof.Index)
		m.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		m.HandleFunc("/debug/pprof/profile", pprof.Profile)
		m.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		m.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
}
