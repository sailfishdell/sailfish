// +build pipe

package main

import (
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/httpcommon"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/rawjsonstream"
)

// nolint: gochecknoinits
// have to have init() function to runtime register the compile-time optional components, dont see any other clean way to do this
func init() {
	initOptional()
	optionalComponents = append([]func(log.Logger, *viper.Viper, busIntf) func(){
		func(logger log.Logger, cfg *viper.Viper, d busIntf) func() {
			serverlist := createHTTPServerBookkeeper(logger)
			addPipeHandlers(logger, cfg, d, serverlist)
			return nil
		}}, optionalComponents...)
}

func addPipeHandlers(logger log.Logger, cfgMgr *viper.Viper, d busIntf, _ *httpcommon.ServerTracker) {
	logger.Crit("PIPE ENABLED")
	cfgMgr.SetDefault("pipe", "/var/run/telemetryservice/metric-engine.pipe")

	pipePaths := cfgMgr.GetStringSlice("pipe")
	if len(pipePaths) == 0 {
		fmt.Fprintf(os.Stderr, "No SSE listeners configured! Use the 'sse' YAML option to configure a listener!")
		return
	}

	// hint to the runtime it can release this memory
	cfgMgr = nil

	for _, pipePath := range pipePaths {
		logger.Crit("Starting up pipe listener", "path", pipePath)
		go rawjsonstream.StartPipeHandler(logger, pipePath, d.GetBus())
	}
}
