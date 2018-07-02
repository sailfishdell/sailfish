package dm_event

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	HealthEvent = eh.EventType("HealthEvent")
	DataManagerEvent = eh.EventType("DataManagerEvent")
)

func init() {
	eh.RegisterEventData(HealthEvent, func() eh.EventData { return &HealthEventData{} })
	eh.RegisterEventData(DataManagerEvent, func() eh.EventData { 
        var f DataManagerEventData
        return f
        })
}

type HealthEventData struct {
    FQDD string
    Health string
}

type DataManagerEventData interface{}



/*
type DataManagerEventData struct {
    FQDD string
    ObjFlags int
    ObjSize int
    ObjStatus int
    ObjType int
    RefreshInterval      int   `json:"refreshInterval"`
    thp_thermal_sensor   interface{} `json:"thp_thermal_sensor" mapstructure:"thp_thermal_sensor"`
    thp_fan_data_object  interface{} `json:"thp_fan_data_object" mapstructure:"thp_thermal_sensor"`
}


type ThpThermalSensor struct {
    SensorReading int  `json:"sensorReading"`
}

type ThpFanDataObject struct {
    Numrotors int `json:"numrotors"`
    Fanpwm int      `json:"fanpwm"`
    Rotor1rpm int       `json:"rotor1rpm"`
    Rotor2rpm int       `json:"rotor2rpm"`
}

    "thp_fan_data_object":{
        "criticalThreshold":1127116133,
        "fanStateMask":825127785,
        "fanhealth":3,
        "fanpwm":1.471363387541058e-42,
        "fanpwm_int":0,
        "numrotors":0,
        "objExtFlags":107,
        "rotor1rpm":1,
        "rotor2rpm":1953724755,
        "warningThreshold":1936941416}}}
*/
