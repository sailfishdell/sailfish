package dm_event

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	HealthEvent                         = eh.EventType("HealthEvent")
	InstPowerEvent                      = eh.EventType("InstPowerEvent")
	DataManagerEvent                    = eh.EventType("DataManagerEvent")
	FanEvent                            = eh.EventType("FanEvent")
	ThermalSensorEvent                  = eh.EventType("ThermalSensorEvent")
	PowerSupplyObjEvent                 = eh.EventType("PowerSupplyObjEvent")
	PowerConsumptionDataObjEvent        = eh.EventType("PowerConsumptionDataObjEvent")
	AvgPowerConsumptionStatDataObjEvent = eh.EventType("AvgPowerConsumptionStatDataObjEvent")
	IomCapability                       = eh.EventType("IomCapability")
	ComponentRemoved                    = eh.EventType("ComponentRemoved")
	FileReadEvent                       = eh.EventType("FileReadEvent")
	FileLinkEvent                       = eh.EventType("FileLinkEvent")
	StorageEnclosureEvent               = eh.EventType("StorageEnclosureEvent")
	StorageAdapterEvent                 = eh.EventType("StorageAdapterEvent")
	StoragePhysicalEvent                = eh.EventType("StoragePhysicalEvent")
	StorageVirtualEvent                 = eh.EventType("StorageVirtualEvent")
	ProbeObjEvent                       = eh.EventType("ProbeObjEvent")
)

func init() {
	eh.RegisterEventData(HealthEvent, func() eh.EventData { return &HealthEventData{} })
	eh.RegisterEventData(InstPowerEvent, func() eh.EventData { return &InstPowerEventData{} })
	eh.RegisterEventData(FanEvent, func() eh.EventData { return &FanEventData{} })
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
	eh.RegisterEventData(StorageAdapterEvent, func() eh.EventData { return &StorageAdapterObjEventData{} })
	eh.RegisterEventData(StorageEnclosureEvent, func() eh.EventData { return &StorageEnclosureObjEventData{} })
	eh.RegisterEventData(StoragePhysicalEvent, func() eh.EventData { return &StoragePhysicalObjEventData{} })
	eh.RegisterEventData(StorageVirtualEvent, func() eh.EventData { return &StorageVirtualObjEventData{} })
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
	FQDD   string
	Health string
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

type StorageVirtualObjEventData struct {
	ObjectHeader        DataObjectHeader
	RaidObjHeader       RaidObjectHeader `mapstructure:"raidObjheader"`
	BlockSize           int              `mapstructure:"blockSize"`
	Capacity            int64            `mapstructure:"size"`
	Encrypted           uint32           `mapstructure:"attributes"`
	OptimumIoSize       int              `mapstructure:"stripeSize"`
	VolumeType          uint32           `mapstructure:"raidLevel"`
	Protocol            int              `mapstructure:"availableProtocols"`
	Cachecade           int              `mapstructure:"attributes"`
	DiskCachePolicy     int              `mapstructure:"diskCachePolicy"`
	LockStatus          int              `mapstructure:"attributes"`
	MediaType           int              `mapstructure:"attributes"`
	ReadCachePolicy     int              `mapstructure:"cachePolicy"`
	SpanDepth           int              `mapstructure:"spanDepth"`
	SpanLength          int              `mapstructure:"pdsPerSpan"`
	VirtualDiskTargetID int              `mapstructure:"targetID"`
	WriteCachePolicy    int              `mapstructure:"cachePolicy"`
	Id                  string           `mapstructure:"fqdd"`
	Description         string
}

type StoragePhysicalObjEventData struct {
	ObjectHeader              DataObjectHeader
	RaidObjHeader             RaidSiObjectHeader `mapstructure:"raidSiobjheader"`
	BlockSize                 int                `mapstructure:"blockSize"`
	Capacity                  uint64             `mapstructure:"size"`
	CapableSpeeds             int                `mapstructure:"capableSpeeds"`
	EncryptionAbility         int                `mapstructure:"attributes"`
	EncryptionStatus          int                `mapstructure:"securityState"`
	FailurePredicted          int                `mapstructure:"attributes"`
	Hotspare                  int                `mapstructure:"hotspare"`
	Id                        string             `mapstructure:"fqdd"`
	Manufacturer              string             `mapstructure:"manufactureName"`
	Model                     string             `mapstructure:"modelName"`
	MediaType                 int                `mapstructure:"attributes"`
	NegotiatedSpeed           int                `mapstructure:"negotiatedSpeed"`
	PartNumber                string             `mapstructure:"ppid"`
	PredictedMediaLife        int                `mapstructure:"deviceLifeRemaining"`
	Protocol                  int                `mapstructure:"protocol"`
	Revision                  string             `mapstructure:"revision"`
	NominalMediumRotationRate int                `mapstructure:"nominalMediumRotationRate"`
	Serial                    string             `mapstructure:"serialNumber"`
	DriveFormFactor           int                `mapstructure:"driveFormFactor"`
	Connector                 int
	FreeSize                  uint64 `mapstructure:"freeSize"`
	ManufacturingDay          uint16 `mapstructure:"manufactureDay"`
	ManufacturingWeek         uint16 `mapstructure:"manufactureWeek"`
	ManufacturingYear         uint32 `mapstructure:"manufactureYear"`
	Ppid                      string `mapstructure:"ppid"`
	PredictiveFailState       int    `mapstructure:"attributes"`
	RaidStatus                int    `mapstructure:"raidState"`
	SasAddress                string `mapstructure:"wwn"`
	Slot                      int8   `mapstructure:"slot"`
	UsedSize                  uint64 `mapstructure:"usedSize"`
}

type StorageAdapterObjEventData struct {
	ObjectHeader                 DataObjectHeader
	RaidObjHeader                RaidSiObjectHeader `mapstructure:"raidSiobjheader"`
	Wwn                          string             `mapstructure:"wwn"`
	Manufacturer                 string             `mapstructure:"manufacturer"`
	FirmwareVersion              string             `mapstructure:"firmwareVersion"`
	Id                           string             `mapstructure:"fqdd"`
	CapableSpeeds                int                `mapstructure:"capableSpeeds"`
	CacheSizeInMb                int                `mapstructure:"cacheSize"`
	CachecadeCapability          int                `mapstructure:"attributes"`
	ControllerFirmwareVersion    string             `mapstructure:"firmwareVersion"`
	DeviceCardSlotType           int                `mapstructure:"slotType"`
	DriverVersion                int                `mapstructure:""`
	EncryptionCapability         int                `mapstructure:"attributes"`
	EncryptionMode               int                `mapstructure:"encryptionmode"`
	PCISlot                      int                `mapstructure:"slot"`
	Embedded                     int                `mapstructure:"embedded"`
	PatrolReadState              int                `mapstructure:"prMode"`
	RollupStatus                 int
	SecurityStatus               int    `mapstructure:"attributes"`
	SlicedVDCapability           int    `mapstructure:"attributes"`
	Model                        string `mapstructure:"fqdd"`
	SupportedDiskProtocols       int    `mapstructure:"supportedDiskProtocols"`
	SupportedControllerProtocols int
}

type StorageEnclosureObjEventData struct {
	ObjectHeader  DataObjectHeader
	RaidObjHeader RaidSiObjectHeader `mapstructure:"raidSiobjheader"`
	AssetTag      string             `mapstructure:"assetTag"`
	ChassisType   int                `json:"bpType"`
	DeviceId      string             `mapstructure:"fqdd"`
	Manufacturer  string             `mapstructure:"manufacturer"`
	Model         string             `mapstructure:"fqdd"`
	PartNumber    string             `mapstructure:"ppid"`
	//PowerState    int                `mapstructure:"drivePower"`
	Sku        string `mapstructure:"serialNumber"`
	Serial     string `mapstructure:"serialNumber"`
	Connector  int    `mapstructure:"port"`
	ServiceTag string `mapstructure:"serviceTag"`
	SlotCount  int    `mapstructure:"slotCount"`
	Version    string `mapstructure:"revision"`
	WiredOrder int    `mapstructure:"position"`
	PowerState int    `mapstructure:"powerState"`
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
	probestatus          int                `json:"probeStatus"`
	probecapabilities    int                `json:"probeCapabilities"`
	objextflags          int                `json:"objExtFlags"`
	offsetkey            string             `mapstructure:"offsetKey"`
	offsetrealiasedname  string             `mapstructure:"offsetReAliasedName"`
	offsetfriendlyFQDD   string             `mapstructure:"offsetFriendlyFQDD"`
	type1maxreadingrange int                `json:"type1MaxReadingRange"`
	type1minreadingrange int                `json:"type1MinReadingRange"`
	unitmodifier         int                `json:"unitModifier"`
	subtype              int                `json:"subType"`
	probereading         int                `json:"probeReading"`
	offsettargetdevkey   string             `mapstructure:"offsetTargetDevKey"`
	sensornumber         int                `mapstructure:"sensorNumber"`
	initupdateinprogress int                `mapstructure:"InitupdateInProgress"`
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
