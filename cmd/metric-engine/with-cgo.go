// +build cgo
// +build idrac_cgo_integration
// +build arm

package main

import (
	"github.com/superchalupa/sailfish/cmd/metric-engine/cgo"
	log "github.com/superchalupa/sailfish/src/log"
)

func cgoStartup(logger log.Logger, d *busComponents) {
	cgo.Startup(logger, d)
}

func cgoShutdown() {
	cgo.Shutdown()
}
