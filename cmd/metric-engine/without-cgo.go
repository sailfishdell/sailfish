// +build !cgo !arm !idrac_cgo_integration

package main

import (
	log "github.com/superchalupa/sailfish/src/log"
)

func cgoStartup(logger log.Logger, d *busComponents) {
	logger.Crit("CGO DISABLED")
}

func cgoShutdown() {
}
