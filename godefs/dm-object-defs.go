// +build ignore

package godefs

/*
// cgo preamble
#cgo pkg-config: libthermalpop dcsgen
#include <thermalpop.h>
*/
import "C"

import (
	eh "github.com/looplab/eventhorizon"
)

// Base DM data definitions
type DM_ObjID C.ObjID

const Sizeof_DM_ObjID = C.sizeof_ObjID

type DM_DataObjHeader C.DataObjHeader

const Sizeof_DM_DataObjHeader = C.sizeof_DataObjHeader

// Fan Data Object
type DM_thp_fan_data_object C.thp_fan_data_object

const Sizeof_DM_thp_fan_data_object = C.sizeof_thp_fan_data_object

func init() {
	eh.RegisterEventData(eh.EventType("thp_fan_data_object"), func() eh.EventData {
		return &DMObject{
			Data: &DM_thp_fan_data_object{},
		}
	})
}
