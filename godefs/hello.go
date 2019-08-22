// +build ignore

package godefs

// #include <hello.h>
import "C"

import (
	eh "github.com/looplab/eventhorizon"
)

type HelloEventData C.hellostruct_t

const Sizeof_HelloEventData = C.sizeof_hellostruct_t

const (
	HelloEvent = eh.EventType("HelloEvent")
)

func init() {
	eh.RegisterEventData(HelloEvent, func() eh.EventData { return &HelloEventData{} })
}
