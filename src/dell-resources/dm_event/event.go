package dm_event

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	HealthEvent      = eh.EventType("HealthEvent")
	DataManagerEvent = eh.EventType("DataManagerEvent")
	FanEvent         = eh.EventType("FanEvent")
	AttributeUpdated = eh.EventType("AttributeUpdated")
	AvgPowerConsumptionStatDataObjEvent = eh.EventType("AvgPowerConsumptionStatDataObjEvent")
)

func init() {
	eh.RegisterEventData(HealthEvent, func() eh.EventData { return &HealthEventData{} })
	eh.RegisterEventData(FanEvent, func() eh.EventData { return &FanEventData{} })
	eh.RegisterEventData(AvgPowerConsumptionStatDataObjEvent, func() eh.EventData { return &AvgPowerConsumptionStatDataObjEventData{} })
	eh.RegisterEventData(AttributeUpdated, func() eh.EventData { return &AttributeUpdatedData{} })
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

type AttributeUpdatedData struct {
	ReqID	string
	FQDD	string
        Health	string
	Group	string
	Index	string
	Name	string
	Value	string
	Error	string
}

type AvgPowerConsumptionStatDataObjEventData struct {
	ObjectHeader	DataObjectHeader
	AvgPwrConsByInterval int `json:"avgPwrConsByInterval"`
	AvgPwrLastDay	int `json:"avgPwrLastDay"`
	AvgPwrLastHour	int `json:"avgPwrLastHour"`
	AvgPwrLastMin	int `json:"avgPwrLastMin"`
	AvgPwrLastWeek	int `json:"avgPwrLastWeek"`
	DefInterval	int `json:"defInterval"`
	DeviceType	int `json:"deviceType"`
	MaxPwrConsByInterval int `json:"maxPwrConsByInterval"`
	MaxPwrLastDay	int `json:"maxPwrLastDay"`
	MaxPwrLastDayTime int64 `json:"maxPwrLastDayTime"`
	MaxPwrLastHour	int `json:"maxPwrLastHour"`
	MaxPwrLastHourTime int64 `json:"maxPwrLastHourTime"`
	MaxPwrLastMin	int `json:"maxPwrLastMin"`
	MaxPwrLastMinTime int64 `json:"maxPwrLastMinTime"`
	MaxPwrLastWeek	int `json:"maxPwrLastWeek"`
	MaxPwrLastWeekTime int64 `json:"maxPwrLastWeekTime"`
	MinPwrConsByInterval int `json:"minPwrConsByInterval"`
	MinPwrLastDay	int `json:"minPwrLastDay"`
	MinPwrLastDayTime int64 `json:"minPwrLastDayTime"`
	MinPwrLastHour	int `json:"minPwrLastHour"`
	MinPwrLastHourTime int64 `json:"minPwrLastHourTime"`
	MinPwrLastMin	int `json:"minPwrLastMin"`
	MinPwrLastMinTime int64 `json:"minPwrLastMinTime"`
	MinPwrLastWeek	int `json:"minPwrLastWeek"`
	MinPwrLastWeekTime int64 `json:"minPwrLastWeekTime"`
	ObjExtFlags	int `json:"objExtFlags"`
	OffsetKey	string `json:"offsetKey"`
}

type DataManagerEventData interface{}
