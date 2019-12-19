package cgo

import (
	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

/*
#include <czmq.h>
#include <zsys.h>
void start_cgo_event_loop();

extern volatile int zsys_interrupted;
extern volatile int zctx_interrupted; // deprecated name

void start_czmq() {
	// disable czmq signal handling as it conflicts with the go signal handling
	printf("FROM CGO: disable czmq signal handling\n");
	zsys_handler_set(NULL);
}

void stop_czmq() {
	zsys_interrupted = 1;
	zctx_interrupted = 1;
}

*/
// #cgo pkg-config: libdds libczmq
import "C"

type BusComponents interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
	GetPublisher() eh.EventPublisher
}

func AddAM3cgo(logger log.Logger, am3Svc *am3.Service, d BusComponents) {
	logger.Crit("CGO ENABLED")
	C.start_czmq()
	go C.start_cgo_event_loop()
}

func Shutdown() {
	C.stop_czmq()
}
