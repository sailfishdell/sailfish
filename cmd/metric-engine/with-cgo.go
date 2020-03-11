// +build cgo
// +build idrac_cgo_integration
// +build arm

package main

import (
	"github.com/superchalupa/sailfish/cmd/metric-engine/cgo"
	log "github.com/superchalupa/sailfish/src/log"
)

// nolint: gochecknoinits
// have to have init() function to runtime register the compile-time optional components, dont see any other clean way to do this
func init() {
	initOptional()
	optionalComponents = append([]func(log.Logger, *viper.Viper, busIntf) func(){
		func(logger log.Logger, cfg *viper.Viper, d busIntf) func() {
			cgoStartup(logger, d)
			return cgoShutdown
		}}, optionalComponents...)
}

func cgoStartup(logger log.Logger, d busIntf) {
	cgo.Startup(logger, d)
}

func cgoShutdown() {
	cgo.Shutdown()
}
