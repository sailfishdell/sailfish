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

func init() {
	initOptional()
	optionalComponents = append([]func(log.Logger, *viper.Viper, *busComponents) func(){
		func(logger log.Logger, cfg *viper.Viper, d *busComponents) func() {
			serverlist := createHttpServerBookkeeper(logger)
			addPprofHandlers(logger, cfg, serverlist)
			return nil
		}}, optionalComponents...)
}

func addPprofHandlers(logger log.Logger, cfgMgr *viper.Viper, serverlist *httpcommon.ServerTracker) {
	logger.Crit("PPROF ENABLED")
	cfgMgr.SetDefault("pprof", "unix:/run/telemetryservice/http.socket")

	listen_addrs := cfgMgr.GetStringSlice("pprof")
	if len(listen_addrs) == 0 {
		fmt.Fprintf(os.Stderr, "No PPROF listeners configured! Use the 'pprof' YAML option to configure a listener!")
		return
	}

	// hint to the runtime it can release this memory
	cfgMgr = nil

	for _, listen := range listen_addrs {
		m := serverlist.GetHandler(listen)

		m.HandleFunc("/debug/pprof/", pprof.Index)
		m.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		m.HandleFunc("/debug/pprof/profile", pprof.Profile)
		m.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		m.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
}
