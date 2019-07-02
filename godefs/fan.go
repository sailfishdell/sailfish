// +build ignore

package godefs

// #cgo CPPFLAGS: -I/home/michael_e_brown/15g/build/14g-EC/xrev/tmp/work/armv7a-poky-linux-gnueabi/libgo-dmobj/1.0+git246+4ffe4f374fb9f9962c3176a03886327d42661673-r1.1/recipe-sysroot/usr/include/libthermalpop
// #cgo pkg-config: libthermalpop
// #include <thermalpop.h>
import "C"

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	eh "github.com/looplab/eventhorizon"
)

type DM_ObjID C.ObjID

const Sizeof_DM_ObjID = C.sizeof_ObjID

type DM_DataObjHeader C.DataObjHeader

const Sizeof_DM_DataObjHeader = C.sizeof_DataObjHeader

type DM_thp_fan_data_object C.thp_fan_data_object

const Sizeof_DM_thp_fan_data_object = C.sizeof_thp_fan_data_object

type myplaceholder struct {
	Data  DM_thp_fan_data_object
	Extra []byte
}

func (mp *myplaceholder) Decode(eventData map[string]interface{}) error {

	fmt.Printf("CUSTOM DECODE\n")

	// TODO: assert string
	structdata, err := base64.StdEncoding.DecodeString(eventData["data"].(string))
	if err != nil {
		fmt.Printf("ERROR decoding base64 event data: %s", err)
		return errors.New("base64 decode fail")
	}

	buf := bytes.NewReader(structdata)
	err = binary.Read(buf, binary.LittleEndian, &mp.Data)

	fmt.Printf("Decode buffer size: %d\n", len(structdata))
	fmt.Printf("Stuct  buffer size: %d\n", binary.Size(mp.Data))

	if binary.Size(mp.Data) < len(structdata) {
		mp.Extra = structdata[binary.Size(mp.Data):]
		fmt.Printf("Saving extra data size(%d) data: %s\n", len(structdata)-binary.Size(mp.Data), mp.Extra)
	}

	return err
}

func init() {
	eh.RegisterEventData(eh.EventType("thp_fan_data_object"), func() eh.EventData { return &myplaceholder{} })
}
