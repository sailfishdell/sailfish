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
			am3SvcN4, _ := am3.StartService(context.Background(), log.With(logger, "module", "Redfish_AM3"), "Redfish", d)
			cfg.SetDefault("redfish", "unix:/run/telemetryservice/http.socket")
			rfListeners := cfg.GetStringSlice("redfish")
			// Startup can block on message registry, so do it in the background
			go func() {
				err := redfish.Startup(logger, am3SvcN4, d)
				if err != nil {
					panic("redfish startup init failed: " + err.Error())
				}
				serverlist := createHTTPServerBookkeeper(logger)
				addRedfishHandlers(logger, rfListeners, d, serverlist)
			}()
			return nil
		}}, optionalComponents...)
}

func addRedfishHandlers(logger log.Logger, listenAddrs []string, d am3.BusObjs, serverlist *httpcommon.ServerTracker) {
	logger.Crit("REDFISH ENABLED")

	if len(listenAddrs) == 0 {
		fmt.Fprintf(os.Stderr, "No REDFISH listeners configured! Use the 'redfish' YAML option to configure a listener!")
		return
	}

	RFS := redfish.NewRedfishServer(logger, d)

	for _, listen := range listenAddrs {
		m := serverlist.GetHandler(listen)
		RFS.AddHandlersToRouter(m)
		logger.Crit("Add redfish routes to handler", "listen", listen, "handler", m)
	}
}
