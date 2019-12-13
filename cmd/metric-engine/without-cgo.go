// +build !cgo !arm

package main

import (
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

func addAM3cgo(logger log.Logger, am3Svc *am3.Service, d *BusComponents) {
	logger.Crit("CGO DISABLED")
}
