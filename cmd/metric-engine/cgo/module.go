package cgo

import (
	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

/*
void start_cgo_event_loop();
*/
// #cgo pkg-config: libdds
import "C"

type BusComponents interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
	GetPublisher() eh.EventPublisher
}

func AddAM3cgo(logger log.Logger, am3Svc *am3.Service, d BusComponents) {
	logger.Crit("CGO ENABLED")
	go C.start_cgo_event_loop()
}
