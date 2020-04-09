// +build redfish

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/httpcommon"
	"github.com/superchalupa/sailfish/cmd/metric-engine/redfish"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

// nolint: gochecknoinits
// have to have init() function to runtime register the compile-time optional components, dont see any other clean way to do this
func init() {
	initOptional()
	optionalComponents = append([]func(log.Logger, *viper.Viper, busIntf) func(){
		func(logger log.Logger, cfg *viper.Viper, d busIntf) func() {
			redfish.RegisterEvents()
			serverlist := createHTTPServerBookkeeper(logger)
			addRedfishHandlers(logger, cfg, d, serverlist)
			am3SvcN4, _ := am3.StartService(context.Background(), log.With(logger, "module", "Redfish_AM3"), "Redfish", d)
			err := redfish.Startup(logger, cfg, am3SvcN4, d)
			if err != nil {
				panic("redfish startup init failed: " + err.Error())
			}
			return nil
		}}, optionalComponents...)
}

func addRedfishHandlers(logger log.Logger, cfgMgr *viper.Viper, d busIntf, serverlist *httpcommon.ServerTracker) {
	logger.Crit("REDFISH ENABLED")
	cfgMgr.SetDefault("redfish", "unix:/run/telemetryservice/http.socket")

	listenAddrs := cfgMgr.GetStringSlice("redfish")
	if len(listenAddrs) == 0 {
		fmt.Fprintf(os.Stderr, "No REDFISH listeners configured! Use the 'redfish' YAML option to configure a listener!")
		return
	}

	// hint to the runtime it can release this memory
	cfgMgr = nil

	RFS := redfish.NewRedfishServer(logger, d)

	for _, listen := range listenAddrs {
		m := serverlist.GetHandler(listen)
		RFS.AddHandlersToRouter(m)
		logger.Crit("Add redfish routes to handler", "listen", listen, "handler", m)
	}
}
