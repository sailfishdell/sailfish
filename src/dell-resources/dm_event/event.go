package dm_event

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	HealthEvent                         = eh.EventType("HealthEvent")
	InstPowerEvent                      = eh.EventType("InstPowerEvent")
	DataManagerEvent                    = eh.EventType("DataManagerEvent")
	ThermalSensorEvent                  = eh.EventType("ThermalSensorEvent")
	PowerSupplyObjEvent                 = eh.EventType("PowerSupplyObjEvent")
	PowerConsumptionDataObjEvent        = eh.EventType("PowerConsumptionDataObjEvent")
	AvgPowerConsumptionStatDataObjEvent = eh.EventType("AvgPowerConsumptionStatDataObjEvent")
	IomCapability                       = eh.EventType("IomCapability")
	ComponentRemoved                    = eh.EventType("ComponentRemoved")
	FileReadEvent                       = eh.EventType("FileReadEvent")
	FileLinkEvent                       = eh.EventType("FileLinkEvent")
	ProbeObjEvent                       = eh.EventType("ProbeObjEvent")
)

func init() {
	eh.RegisterEventData(HealthEvent, func() eh.EventData { return &HealthEventData{} })
	eh.RegisterEventData(InstPowerEvent, func() eh.EventData { return &InstPowerEventData{} })
	eh.RegisterEventData(ThermalSensorEvent, func() eh.EventData { return &ThermalSensorEventData{} })
	eh.RegisterEventData(PowerSupplyObjEvent, func() eh.EventData { return &PowerSupplyObjEventData{} })
	eh.RegisterEventData(PowerConsumptionDataObjEvent, func() eh.EventData { return &PowerConsumptionDataObjEventData{} })
	eh.RegisterEventData(AvgPowerConsumptionStatDataObjEvent, func() eh.EventData { return &AvgPowerConsumptionStatDataObjEventData{} })
	eh.RegisterEventData(FileReadEvent, func() eh.EventData { return &FileReadEventData{} })
	eh.RegisterEventData(FileLinkEvent, func() eh.EventData { return &FileLinkEventData{} })
	eh.RegisterEventData(DataManagerEvent, func() eh.EventData {
		var f DataManagerEventData
		return f
	})
	eh.RegisterEventData(IomCapability, func() eh.EventData { return &IomCapabilityData{} })
	eh.RegisterEventData(ComponentRemoved, func() eh.EventData { return &ComponentRemovedData{} })
	eh.RegisterEventData(ProbeObjEvent, func() eh.EventData { return &ProbeObjEventData{} })
}

type IomCapabilityData struct {
	Name                    string
	Internal_mgmt_supported bool
	CapabilitiesCount       int
	Capabilities            interface{}
	IOMConfig_objects       interface{}
}

type ComponentRemovedData struct {
	Name string
}

type HealthEventData struct {
	FQDD     string
	Health   string
	EventSeq int64 `mapstructure:"event_seq"`
}
type InstPowerEventData struct {
	FQDD      string
	InstPower float64
}

type DataObjectHeader struct {
	ObjStatus       int `mapstructure:"objStatus"`
	ObjSize         int `mapstructure:"objSize"`
	ObjType         int `mapstructure:"objType"`
	RefreshInterval int `mapstructure:"refreshInterval"`
	ObjFlags        int `mapstructure:"objFlags"`
	FQDD            string
	Struct          string
}

type ThermalSensorEventData struct {
	ObjectHeader           DataObjectHeader
	ReadingValid           int    `json:"readingValid"`
	UpperCriticalThreshold int    `json:"upperCriticalThreshold"`
	SensorReading          int    `json:"sensorReadingInt"`
	LowerWarningThreshold  int    `json:"lowerWarningThreshold"`
	SensorStateMask        int    `json:"sensorStateMask"`
	SensorHealth           int    `json:"sensorHealth"`
	UpperWarningThreshold  int    `json:"upperWarningThreshold"`
	LowerCriticalThreshold int    `json:"lowerCriticalThreshold"`
	OffsetDeviceName       string `json:"offsetDeviceName"`
	OffsetDeviceFQDD       string `json:"offsetDeviceFQDD"`
	OffsetKey              string `json:"offsetKey"`
	OffsetSensorName       string `json:"offsetSensorName"`
	OffsetVendorName       string `json:"offsetVendorName"`
}
type BaseSoftwareInventoryObjectobj struct {
	OffsetVersionDependencyArray string `json:"OffsetVersionDependencyArray"`
	DeviceType                   int    `json:"DeviceType"`
	FQDDOffset                   string `json:"FQDDOffset"`
}
type SlotBasedSoftwareInventoryObjectobj struct {
	BaseSoftwareInventoryObject BaseSoftwareInventoryObjectobj `json:"BaseSoftwareInventoryObject"`
	SlotNum                     int                            `json:"SlotNum"`
	UniqueID                    int                            `json:"UniqueID"`
}
type PowerSupplyObjEventData struct {
	ObjectHeader                     DataObjectHeader
	SlotBasedSoftwareInventoryObject SlotBasedSoftwareInventoryObjectobj `json:"slotBasedSoftwareInventoryObject"`
	OutputWatts                      int                                 `json:"outputWatts"`
	InputRatedWatts                  int                                 `json:"inputRatedWatts"`
	InputVolts                       int                                 `json:"inputVolts"`
	PSACOn                           float64                             `json:"psACOn"`
	PSSwitchOn                       float64                             `json:"psSwitchOn"`
	PSPOK                            float64                             `json:"psPOK"`
	PSOn                             float64                             `json:"psOn"`
	PSFanFail                        float64                             `json:"psFanFail"`
	PSState                          uint16                              `json:"psState"`
	PSType                           uint8                               `json:"psType"`
	PSCfgErrType                     uint8                               `json:"psCfgErrType"`
	BPMCapable                       float64                             `json:"bPMCapable"`
	RatedAmps                        uint16                              `json:"ratedAmps"`
	InputStatus                      uint8                               `json:"inputStatus"`
	PsuSlot                          uint8                               `json:"psuSlot"`
	InstAmps                         float64                             `json:"instAmps"`
	PsuCapabilities                  uint                                `json:"psuCapabilities"`
	OffsetFwVer                      string                              `json:"offsetfwVer"`
	OffsetPSLocation                 string                              `json:"offsetPSLocation"`
	BoardProductName                 string                              `json:"boardProductName"`
	BoardSerialNumber                string                              `json:"boardSerialNumber"`
	BoardPartNumber                  string                              `json:"boardPartNumber"`
	BoardManufacturer                string                              `json:"boardManufacturer"`
	RedundancyStatus                 uint8                               `json:"redundancyStatus"`
	UpdateTime                       int
	CurrentInputVolts                int    `json:"currentInputVolts"`
	MinimumVoltage                   uint16 `json:"minimumvoltage"`
	MaximumVoltage                   uint16 `json:"maxmimumvoltage"`
	MinimumFreqHz                    uint8  `json:"minimumfreqhz"`
	MaximumFreqHz                    uint8  `json:"maximumfreqhz"`
	InitUpdateInProgress             uint   `json:"InitupdateInProgress"`
	U16POutMax                       uint16 `json:"u16PoutMax"`
	LineStatus                       uint8  `json:"lineStatus"`
}

type PowerConsumptionDataObjEventData struct {
	ObjectHeader    DataObjectHeader
	CwStartTime     int64 `json:"cwStartTime"`
	CumulativeWatts int   `json:"cumulativeWatts"`
	InstHeadRoom    int   `json:"instHeadRoom"`
	PeakWatts       int   `json:"peakWatts"`
	PwReadingTime   int64 `json:"pwReadingTime"`
	MinWatts        int   `json:"minWatts"`
	MinwReadingTime int64 `json:"minwReadingTime"`
	PeakHeadRoom    int   `json:"peakHeadRoom"`
	Maxpower        int   `json:"maxPower"`
	InstWattsPSU1_2 int   `json:"instWattsPSU1_2"`
}

type AvgPowerConsumptionStatDataObjEventData struct {
	ObjectHeader         DataObjectHeader
	AvgPwrConsByInterval int    `json:"avgPwrConsByInterval"`
	AvgPwrLastDay        int    `json:"avgPwrLastDay"`
	AvgPwrLastHour       int    `json:"avgPwrLastHour"`
	AvgPwrLastMin        int    `json:"avgPwrLastMin"`
	AvgPwrLastWeek       int    `json:"avgPwrLastWeek"`
	DefInterval          int    `json:"defInterval"`
	DeviceType           int    `json:"deviceType"`
	MaxPwrConsByInterval int    `json:"maxPwrConsByInterval"`
	MaxPwrLastDay        int    `json:"maxPwrLastDay"`
	MaxPwrLastDayTime    int64  `json:"maxPwrLastDayTime"`
	MaxPwrLastHour       int    `json:"maxPwrLastHour"`
	MaxPwrLastHourTime   int64  `json:"maxPwrLastHourTime"`
	MaxPwrLastMin        int    `json:"maxPwrLastMin"`
	MaxPwrLastMinTime    int64  `json:"maxPwrLastMinTime"`
	MaxPwrLastWeek       int    `json:"maxPwrLastWeek"`
	MaxPwrLastWeekTime   int64  `json:"maxPwrLastWeekTime"`
	MinPwrConsByInterval int    `json:"minPwrConsByInterval"`
	MinPwrLastDay        int    `json:"minPwrLastDay"`
	MinPwrLastDayTime    int64  `json:"minPwrLastDayTime"`
	MinPwrLastHour       int    `json:"minPwrLastHour"`
	MinPwrLastHourTime   int64  `json:"minPwrLastHourTime"`
	MinPwrLastMin        int    `json:"minPwrLastMin"`
	MinPwrLastMinTime    int64  `json:"minPwrLastMinTime"`
	MinPwrLastWeek       int    `json:"minPwrLastWeek"`
	MinPwrLastWeekTime   int64  `json:"minPwrLastWeekTime"`
	ObjExtFlags          int    `json:"objExtFlags"`
	OffsetKey            string `json:"offsetKey"`
}

type RaidSiObjectHeader struct {
	ObjVersion          int     `json:"objVersion"`
	ObjName             string  `json:"objName"`
	ObjAttributes       int     `json:"objAttributes"`
	PrimaryStatus       int     `json:"primaryStatus"`
	UpdateTime          int     `json:"updateTime"`
	SyncTime            float64 `json:"syncTime"`
	AlternateFQDDOffset int     `json:"alternateFQDDOffset"`
	Flags               int     `json:"flags"`
}

type RaidObjectHeader struct {
	DataObjHeader       DataObjectHeader
	PrimaryStatus       int     `json:"primaryStatus"`
	FriendlyFQDDOffset  int     `json:"friendlyFQDDOffset"`
	SyncTime            float64 `json:"syncTime"`
	ObjExtFlags         int     `json:"objExtFlags"`
	ObjAttributes       int     `json:"objAttributes"`
	ObjName             string  `json:"objName"`
	KeyOffset           int     `json:"keyOffset"`
	ObjVersion          int     `json:"objVersion"`
	FqddOffset          int     `json:"fqddOffset"`
	AlternateFQDDOffset int     `json:"alternateFQDDOffset"`
	Flags               int     `json:"flags"`
	UpdateTime          int     `json:"updateTime"`
}

type ProbeThresholdsobj struct {
	UnrThreshold int `json:"unrThreshold"`
	UcThreshold  int `json:"ucThreshold"`
	LncThreshold int `json:"lncThreshold"`
	UncThreshold int `json:"uncThreshold"`
	LcThreshold  int `json:"lcThreshold"`
	LnrThreshold int `json:"lnrThreshold"`
}

type ProbeObjEventData struct {
	ObjectHeader         DataObjectHeader
	ProbeThresholds      ProbeThresholdsobj `json:"ProbeThresholds"`
	OffsetProbeLocation  string             `json:"offsetProbeLocation"`
	EntityID             int                `json:"entityID"`
	ProbeStatus          int                `json:"probeStatus"`
	ProbeCapabilities    int                `json:"probeCapabilities"`
	ObjExtFlags          int                `json:"objExtFlags"`
	OffsetKey            string             `json:"offsetKey"`
	OffsetReAliasedName  string             `json:"offsetReAliasedName"`
	OffsetFriendlyFQDD   string             `json:"offsetFriendlyFQDD"`
	Type1MaxReadingRange int                `json:"type1MaxReadingRange"`
	Type1MinReadingRange int                `json:"type1MinReadingRange"`
	UnitModifier         int                `json:"unitModifier"`
	SubType              int                `json:"subType"`
	ProbeReading         int                `json:"probeReading"`
	SensorNumber         int                `json:"sensorNumber"`
	OffsetTargetDevKey   string             `json:"offsetTargetDevKey"`
	InitupdateInProgress int                `json:"InitupdateInProgress"`
}

type VoltageSensorObjEventData struct {
	ObjectHeader         DataObjectHeader
	ProbeThresholds      ProbeThresholdsobj //`json:"ProbeThresholds"`
	probestatus          int
	probecapabilities    int
	objextflags          int
	offsetkey            string `mapstructure:"offsetKey"`
	offsetrealiasedname  string `mapstructure:"offsetReAliasedName"`
	offsetfriendlyFQDD   string `mapstructure:"offsetFriendlyFQDD"`
	type1maxreadingrange int
	type1minreadingrange int
	unitmodifier         int
	subtype              int
	probereading         int
	offsettargetdevkey   string `mapstructure:"offsetTargetDevKey"`
	sensornumber         int    `mapstructure:"sensorNumber"`
	initupdateinprogress int    `mapstructure:"InitupdateInProgress"`
}

type FileReadEventData struct {
	Content string
	URI     string
	FQDD    string
}

type FileLinkEventData struct {
	FilePath string
	URI      string
	FQDD     string
}

type DataManagerEventData interface{}
