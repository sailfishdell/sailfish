// +build cgo
// +build idrac_cgo_integration
// +build arm

package main

import (
	"github.com/superchalupa/sailfish/cmd/metric-engine/cgo"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

func addAM3cgo(logger log.Logger, am3Svc *am3.Service, d *BusComponents) {
	cgo.AddAM3cgo(logger, am3Svc, d)
}

func cgoShutdown() {
	cgo.Shutdown()
}
