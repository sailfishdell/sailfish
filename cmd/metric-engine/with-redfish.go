// +build redfish

package main

import (
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/httpcommon"
	"github.com/superchalupa/sailfish/cmd/metric-engine/redfish"
	log "github.com/superchalupa/sailfish/src/log"
)

func init() {
	initOptional()
	optionalComponents = append([]func(log.Logger, *viper.Viper, *busComponents) func(){
		func(logger log.Logger, cfg *viper.Viper, d *busComponents) func() {
			serverlist := createHttpServerBookkeeper(logger)
			addRedfishHandlers(logger, cfg, d, serverlist)
			return nil
		}}, optionalComponents...)
}

func addRedfishHandlers(logger log.Logger, cfgMgr *viper.Viper, d *busComponents, serverlist *httpcommon.ServerTracker) {
	logger.Crit("REDFISH ENABLED")
	cfgMgr.SetDefault("redfish", "unix:/run/telemetryservice/http.socket")

	listen_addrs := cfgMgr.GetStringSlice("redfish")
	if len(listen_addrs) == 0 {
		fmt.Fprintf(os.Stderr, "No REDFISH listeners configured! Use the 'redfish' YAML option to configure a listener!")
		return
	}

	// hint to the runtime it can release this memory
	cfgMgr = nil

	RFS := redfish.NewRedfishServer(logger, d)

	for _, listen := range listen_addrs {
		m := serverlist.GetHandler(listen)
		RFS.AddHandlersToRouter(m)
	}
}
