package dm_event

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	HealthEvent                         = eh.EventType("HealthEvent")
	DataManagerEvent                    = eh.EventType("DataManagerEvent")
	FanEvent                            = eh.EventType("FanEvent")
	PowerSupplyObjEvent                 = eh.EventType("PowerSupplyObjEvent")
	PowerConsumptionDataObjEvent        = eh.EventType("PowerConsumptionDataObjEvent")
	AvgPowerConsumptionStatDataObjEvent = eh.EventType("AvgPowerConsumptionStatDataObjEvent")
	IomCapability                       = eh.EventType("IomCapability")
	FileReadEvent                       = eh.EventType("FileReadEvent")
	FileLinkEvent                       = eh.EventType("FileLinkEvent")
	StorageEnclosureEvent               = eh.EventType("StorageEnclosureEvent")
	StorageAdapterEvent                 = eh.EventType("StorageAdapterEvent")
	StoragePhysicalEvent                = eh.EventType("StoragePhysicalEvent")
	StorageVirtualEvent                 = eh.EventType("StorageVirtualEvent")
)

func init() {
	eh.RegisterEventData(HealthEvent, func() eh.EventData { return &HealthEventData{} })
	eh.RegisterEventData(FanEvent, func() eh.EventData { return &FanEventData{} })
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
	eh.RegisterEventData(StorageAdapterEvent, func() eh.EventData { return &StorageAdapterObjEventData{} })
	eh.RegisterEventData(StorageEnclosureEvent, func() eh.EventData { return &StorageEnclosureObjEventData{} })
	eh.RegisterEventData(StoragePhysicalEvent, func() eh.EventData { return &StoragePhysicalObjEventData{} })
	eh.RegisterEventData(StorageVirtualEvent, func() eh.EventData { return &StorageVirtualObjEventData{} })
}

type IomCapabilityData struct {
	Name                    string
	Internal_mgmt_supported bool
	CapabilitiesCount       int
	Capabilities            interface{}
	IOMConfig_objects       interface{}
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

type PowerSupplyObjEventData struct {
	ObjectHeader         DataObjectHeader
	OutputWatts          int     `json:"outputWatts"`
	InputRatedWatts      int     `json:"inputRatedWatts"`
	InputVolts           int     `json:"inputVolts"`
	PSACOn               float64 `json:"psACOn"`
	PSSwitchOn           float64 `json:"psSwitchOn"`
	PSPOK                float64 `json:"psPOK"`
	PSOn                 float64 `json:"psOn"`
	PSFanFail            float64 `json:"psFanFail"`
	PSState              uint16  `json:"psState"`
	PSType               uint8   `json:"psType"`
	PSCfgErrType         uint8   `json:"psCfgErrType"`
	BPMCapable           float64 `json:"bPMCapable"`
	RatedAmps            uint16  `json:"ratedAmps"`
	InputStatus          uint8   `json:"inputStatus"`
	PsuSlot              uint8   `json:"psuSlot"`
	InstAmps             float64 `json:"instAmps"`
	PsuCapabilities      uint    `json:"psuCapabilities"`
	OffsetFwVer          string  `json:"offsetfwVer"`
	OffsetPSLocation     string  `json:"offsetPSLocation"`
	BoardProductName     string  `json:"boardProductName"`
	BoardSerialNumber    string  `json:"boardSerialNumber"`
	BoardPartNumber      string  `json:"boardPartNumber"`
	BoardManufacturer    string  `json:"boardManufacturer"`
	RedundancyStatus     uint8   `json:"redundancyStatus"`
	UpdateTime           int
	CurrentInputVolts    int    `json:"currentInputVolts"`
	MinimumVoltage       uint16 `json:"minimumvoltage"`
	MaximumVoltage       uint16 `json:"maxmimumvoltage"`
	MinimumFreqHz        uint8  `json:"minimumfreqhz"`
	MaximumFreqHz        uint8  `json:"maximumfreqhz"`
	InitUpdateInProgress uint   `json:"InitupdateInProgress"`
	U16POutMax           uint16 `json:"u16PoutMax"`
	LineStatus           uint8  `json:"lineStatus"`
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

type StorageVirtualObjEventData struct {
	ObjectHeader  DataObjectHeader
	RaidObjHeader RaidObjectHeader `mapstructure:"raidObjheader"`
	BlockSize     int              `json:"blockSize"`
	Capacity      int64            `json:"size"`
	Encrypted     int              `json:"attributes"`
	OptimumIoSize int              `json:"stripeSize"`
	VolumeType    int              `json:"raidLevel"`
	Id            string
	Description   string
}

type StoragePhysicalObjEventData struct {
	ObjectHeader              DataObjectHeader
	RaidObjHeader             RaidSiObjectHeader `mapstructure:"raidSiobjheader"`
	BlockSize                 int                `json:"blockSize"`
	Capacity                  uint64             `json:"size"`
	CapableSpeeds             int                `json:"capableSpeeds"`
	EncryptionAbility         int                `json:"attributes"`
	EncryptionStatus          int                `json:"securityState"`
	FailurePredicted          int                `json:"attributes"`
	Hotspare                  int                `json:"hotspare"`
	Id                        string             `mapstructure:"fqdd"`
	Manufacturer              string             `mapstructure:"manufactureName"`
	Model                     string             `mapstructure:"modelName"`
	MediaType                 int                `json:"attributes"`
	NegotiatedSpeed           int                `json:"negotiatedSpeed"`
	PartNumber                string             `mapstructure:"ppid"`
	PredictedMediaLife        int                `json:"deviceLifeRemaining"`
	Protocol                  int                `json:"protocol"`
	Revision                  string             `mapstructure:"revision"`
	NominalMediumRotationRate int                `json:"nominalMediumRotationRate"`
	Serial                    string             `mapstructure:"serialNumber"`
}

type StorageAdapterObjEventData struct {
	ObjectHeader    DataObjectHeader
	RaidObjHeader   RaidSiObjectHeader `json:"raidSiobjheader"`
	Manufacturer    string             `mapstructure:"manufacturer"`
	FirmwareVersion string             `mapstructure:"currentAvailableFwVer"`
	Id              string             `mapstructure:"fqdd"`
	CapableSpeeds   int                `json:"capableSpeeds"`
	Model           string             `mapstructure:"fqdd"`
}

type StorageEnclosureObjEventData struct {
	ObjectHeader  DataObjectHeader
	RaidObjHeader RaidSiObjectHeader `json:"raidSiobjheader"`
	AssetTag      string             `json:"assetTag"`
	ChassisType   int                `json:"bpType"`
	DeviceId      int                `json:"deviceID"`
	Manufacturer  string             `mapstructure:"manufacturer"`
	Model         string             `mapstructure:"fqdd"`
	PartNumber    string             `mapstructure:"ppid"`
	PowerState    int                `mapstructure:"drivePower"`
	Sku           string             `mapstructure:"serialNumber"`
	Serial        string             `mapstructure:"serialNumber"`
	Connector     int                `json:"connectorCount"`
	ServiceTag    string             `mapstructure:"serviceTag"`
	SlotCount     int                `json:"slotCount"`
	Version       string             `mapstructure:"currentAvailableFwVer"`
	WiredOrder    string             `mapstructure:"currentAvailableFwVer"`
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
