package dm_event

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	HealthEvent      = eh.EventType("HealthEvent")
	DataManagerEvent = eh.EventType("DataManagerEvent")
	FanEvent         = eh.EventType("FanEvent")
)

func init() {
	eh.RegisterEventData(HealthEvent, func() eh.EventData { return &HealthEventData{} })
	eh.RegisterEventData(FanEvent, func() eh.EventData { return &FanEventData{} })
	eh.RegisterEventData(DataManagerEvent, func() eh.EventData {
		var f DataManagerEventData
		return f
	})
}

type HealthEventData struct {
	FQDD   string
	Health string
}

type DataObjectHeader struct {
	ObjStatus       int `json:"objStatus"`
	ObjSize         int `json:"objSize"`
	ObjType         int `json:"objType"`
	RefreshInterval int `json:"refreshInterval"`
	ObjFlags        int `json:"objFlags"`
	FQDD            string
	Struct          string
}

type FanEventData struct {
	ObjectHeader      DataObjectHeader
	Fanpwm            float64 `json:"fanpwm"`
	Key               string
	FanName           string
	Fanpwm_int        int `json:"fanpwm_int"`
	VendorName        string
	WarningThreshold  int `json:"warningThreshold"`
	DeviceName        string
	TachName          string
	CriticalThreshold int `json:"criticalThreshold"`
	Fanhealth         int `json:"fanhealth"`
	Numrotors         int `json:"numrotors"`
	Rotor2rpm         int `json:"rotor2rpm"`
	Rotor1rpm         int `json:"rotor1rpm"`
	FanStateMask      int `json:"fanStateMask"`
}

type DataManagerEventData interface{}
