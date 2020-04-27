package dell_ec

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
	"github.com/superchalupa/sailfish/src/ocp/telemetryservice"

	"github.com/superchalupa/sailfish/godefs"
	"github.com/superchalupa/sailfish/src/dell-resources/attributedef"
	"github.com/superchalupa/sailfish/src/log"
	ev "github.com/superchalupa/sailfish/src/looplab/event"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type Collection struct {
	Prefix      string
	URI         string
	Property    string
	Format      string
	SpecialCase string
}

type AttributeLink struct {
	FQDD          string
	FullURI       string
	CollectionURI string
	Property      string
}

var collections []Collection
var attributeLinks []AttributeLink

func addAM3Functions(logger log.Logger, am3Svc *am3.Service, d *domain.DomainObjects, ctx context.Context) {
	am3Svc.AddEventHandler("modular_update_fan_data", godefs.FanEvent, func(event eh.Event) {
		dmobj, ok1 := event.Data().(*godefs.DMObject)
		fanobj, ok2 := dmobj.Data.(*godefs.DM_thp_fan_data_object)
		if !ok1 || !ok2 {
			logger.Error("updateFanData did not have fan event", "type", event.EventType, "data", event.Data())
			return
		}

		FullFQDD, err := dmobj.GetStringFromOffset(int(fanobj.OffsetKey))
		if err != nil {
			logger.Error("Got an thp_fan_data_object that somehow didn't have a string for FQDD.", "err", err)
			return
		}

		FQDDParts := strings.SplitN(FullFQDD, "#", 2)
		if len(FQDDParts) < 2 {
			logger.Error("Got an thp_fan_data_object with an FQDD that had no hash. Shouldnt happen", "FQDD", FullFQDD)
			return
		}

		URI := "/redfish/v1/Chassis/" + FQDDParts[0] + "/Sensors/Fans/" + FQDDParts[1]
		uuid, ok := d.GetAggregateIDOK(URI)
		if !ok {
			logger.Error("Got an thp_fan_data_object for something that doesn't appear in our redfish tree", "FQDD", FullFQDD, "Calculated URI", URI)
			return
		}

		d.CommandHandler.HandleCommand(context.Background(),
			&domain.UpdateRedfishResourceProperties2{
				ID: uuid,
				Properties: map[string]interface{}{
					"Reading":     (fanobj.Rotor1rpm + fanobj.Rotor2rpm) / 2,
					"Oem/Reading": fanobj.Int,
				},
			})
	})

	MD := "/redfish/v1/TelemetryService/MetricDefinitions/"
	am3Svc.AddEventHandler("add_MD", telemetryservice.AddMDEvent, func(event eh.Event) {
		mdobj, ok1 := event.Data().(*telemetryservice.AddMDData)
		if !ok1 {
			logger.Error("AddMDEvent did not have AddMDData", "type", event.EventType, "data", event.Data())
			return
		}

		wcMaps := []map[string]interface{}{}
		for i := 0; i < len(mdobj.Wildcards); i++ {
			wcMaps = append(wcMaps, map[string]interface{}{"Name": mdobj.Wildcards[i].Name, "Values": mdobj.Wildcards[i].Values})
		}
		uuid := eh.NewUUID()

		d.CommandHandler.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          uuid,
				Type:        "#TelemetryService.v1_0_0.TelemetryService",
				ResourceURI: MD + mdobj.Id,
				Context:     "/redfish/v1/$metadata#MetricDefinition.MetricDefinition",
				Privileges: map[string]interface{}{
					"GET": []string{"Login"},
				},
				Properties: map[string]interface{}{
					"Accuracy":         mdobj.Accuracy,
					"Calibration":      mdobj.Calibration,
					"Id":               mdobj.Id,
					"Implementation":   mdobj.Implementation,
					"MaxReadingRange":  mdobj.MaxReadingRange,
					"MetricDataType":   mdobj.MetricDataType,
					"MetricProperties": mdobj.MetricProperties,
					"MetricType":       mdobj.MetricType,
					"MinReadingRange":  mdobj.MinReadingRange,
					"Name":             mdobj.Name,
					"Wildcards":        wcMaps,
				}})
	})

	am3Svc.AddEventHandler("MMHealthAlertFn", dm_event.HealthEvent, func(event eh.Event) {
		data, ok := event.Data().(*dm_event.HealthEventData)
		if !ok {
			logger.Error("Health  event did not have health event data", "type", event.EventType, "data", event.Data())
			return
		}
		FQDD := data.FQDD

		if FQDD != "Root" {
			return
		}

		Health := data.Health
		SubSystem := "MM"

		t := time.Now()
		cTime := t.Format("2006-01-02T15:04:05-07:00")
		ma := []string{Health}

		//Create Alert type event:
		d.EventBus.PublishEvent(context.Background(),
			eh.NewEvent(eventservice.RedfishEvent, &eventservice.RedfishEventData{
				EventType:         "Alert",
				EventId:           "1",
				EventTimestamp:    cTime,
				Severity:          "Informational",
				Message:           "The chassis health is " + Health,
				MessageId:         "CMC8550",
				MessageArgs:       ma,
				OriginOfCondition: SubSystem,
			}, time.Now()))

	})

	am3Svc.AddEventHandler("AvgPowerConsumptionFn", dm_event.AvgPowerConsumptionStatDataObjEvent, func(event eh.Event) {
		data, ok := event.Data().(*dm_event.AvgPowerConsumptionStatDataObjEventData)
		if !ok {
			logger.Error("Avg Power consumption data event did not have power consumption data", "type", event.EventType, "data", event.Data())
			return
		}

		trendList := []string{"Hour", "Day", "Week"}
		FQDD := data.ObjectHeader.FQDD
		FQDDParts := strings.SplitN(FQDD, "#", 2)
		if len(FQDDParts) < 2 {
			logger.Error("Got an avg powerconsumption data object with an FQDD that had no hash. Shouldnt happen", "FQDD", FQDD)
			return
		}

		for _, trend := range trendList {
			URI := "/redfish/v1/Chassis/" + FQDDParts[0] + "/Power/PowerTrends-1/Last" + trend
			uuid, ok := d.GetAggregateIDOK(URI)
			if !ok {
				logger.Error("aggregate does not exist at URI to update", "URI", URI)
				continue
			}
			d.CommandHandler.HandleCommand(context.Background(),
				&domain.UpdateRedfishResourceProperties2{
					ID: uuid,
					Properties: map[string]interface{}{
						"HistoryMaxWattsTime": interface_to_date(traverse_struct(data, "MaxPwrLast"+trend+"Time")),
						"HistoryMinWattsTime": interface_to_date(traverse_struct(data, "MinPwrLast"+trend+"Time")),
						"HistoryMinWatts":     traverse_struct(data, "MinPwrLast"+trend),
						"HistoryMaxWatts":     traverse_struct(data, "MaxPwrLast"+trend),
						"HistoryAverageWatts": traverse_struct(data, "AvgPwrLast"+trend),
					},
				})
		}
	})

	am3Svc.AddEventHandler("PowerConsumptionFn", dm_event.PowerConsumptionDataObjEvent, func(event eh.Event) {
		data, ok := event.Data().(*dm_event.PowerConsumptionDataObjEventData)
		if !ok {
			logger.Error("Powerconsumptiondata event did not have power consumption data", "type", event.EventType, "data", event.Data())
			return
		}

		FQDD := data.ObjectHeader.FQDD
		FQDDParts := strings.SplitN(FQDD, "#", 2)
		if len(FQDDParts) < 2 {
			logger.Error("Got a powerconsumption data object with an FQDD that had no hash. Shouldnt happen", "FQDD", FQDD)
			return
		}

		URI := "/redfish/v1/Chassis/" + FQDDParts[0] + "/Power/PowerControl"
		uuid, ok := d.GetAggregateIDOK(URI)
		if !ok {
			logger.Error("aggregate does not exist at URI to update", "URI", URI)
			return
		}
		d.CommandHandler.HandleCommand(context.Background(),
			&domain.UpdateRedfishResourceProperties2{
				ID: uuid,
				Properties: map[string]interface{}{
					"Oem/EnergyConsumptionStartTime": epoch2Date(int64(data.CwStartTime)),
					"Oem/EnergyConsumptionkWh":       int(data.CumulativeWatts / 1000),
					"Oem/MaxPeakWatts":               data.PeakWatts,
					"Oem/MaxPeakWattsTime":           epoch2Date(int64(data.PwReadingTime)),
					"Oem/MinPeakWatts":               data.MinWatts,
					"Oem/MinPeakWattsTime":           epoch2Date(int64(data.MinwReadingTime)),
					"Oem/PeakHeadroomWatts":          data.PeakHeadRoom,
					"Oem/HeadroomWatts":              data.InstHeadRoom,
					"PowerConsumedWatts":             data.InstWattsPSU1_2,
					"PowerAvailableWatts":            data.PeakHeadRoom,
				},
			})
	})

	am3Svc.AddEventHandler("ComponentRemovedFn", dm_event.ComponentRemoved, func(event eh.Event) {
		data, ok := event.Data().(*dm_event.ComponentRemovedData)
		if !ok {
			logger.Error("Component Removed event did not have remove event data", "type", event.EventType, "data", event.Data())
			return
		}
		URI := "/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth"
		uuid, ok := d.GetAggregateIDOK(URI)
		if !ok {
			logger.Error("aggregate does not exist at URI to update", "URI", URI)
			return
		}
		subsys := data.Name
		d.CommandHandler.HandleCommand(context.Background(),
			&domain.RemoveRedfishResourceProperty{
				ID:       uuid,
				Property: subsys})
	})

	am3Svc.AddEventHandler("InstPowerFn", dm_event.InstPowerEvent, func(event eh.Event) {
		data, ok := event.Data().(*dm_event.InstPowerEventData)
		if !ok {
			logger.Error("InstPowerEventData did not have event data", "type", event.EventType, "data", event.Data())
			return
		}
		FQDD := data.FQDD
		arr := strings.Split(FQDD, "#")
		// InstPowerEvents handle System.Modular.X power only
		// AttributeUpdated handles IOM inst power
		if len(arr) != 1 {
			FQDD = arr[1]
		}
		pwr := data.InstPower

		URI := "/redfish/v1/Chassis/" + FQDD
		uuid, ok := d.GetAggregateIDOK(URI)
		if !ok {
			logger.Error("aggregate does not exist at URI to update", "URI", URI)
			return
		}

		d.CommandHandler.HandleCommand(context.Background(),
			&domain.UpdateRedfishResourceProperties2{
				ID: uuid,
				Properties: map[string]interface{}{
					"Oem/Dell/InstPowerConsumption": pwr,
				},
			})
	})

	powercap_enabled := false
	am3Svc.AddEventHandler("Am3AttributeUpdatedFn", attributedef.AttributeUpdated, func(event eh.Event) {
		data, ok := event.Data().(*attributedef.AttributeUpdatedData)
		if !ok {
			logger.Error("Attribute Updated Event did not have event data", "type", event.EventType, "data", event.Data())
			return
		}
		URI := ""
		FQDD := data.FQDD
		name := data.Name
		value := data.Value
		key := ""
		switch {

		case isRowOrColumn(FQDD, name, "Rows"):
			value, ok = value_to_string(value)
			if !ok {
				logger.Error("data", "value", value, "parsed", ok)
			}

			key = "Rows"
			URI = "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig." + data.Index
			break

		case isRowOrColumn(FQDD, name, "Columns"):
			value, ok = value_to_string(value)
			if !ok {
				logger.Error("data", "value", value, "parsed", ok)
			}
			key = "Columns"
			URI = "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig." + data.Index
			break

		case isSledProfile(FQDD, name):
			key = "SledProfile"
			FQDDParts := strings.SplitN(FQDD, ".", 3)
			if len(FQDDParts) < 3 {
				logger.Error("Got a wrong sled profile FQDD", "FQDD", FQDD)
				return
			}
			// FQDDParts will be like ["System", "Modular", "1a"]
			// More Validity checks
			if len(FQDDParts[2]) > 1 {
				return
			}
			if _, err := strconv.Atoi(FQDDParts[2]); err != nil {
				return
			}

			URI = "/redfish/v1/Chassis/System.Chassis.1/Slots/SledSlot." + FQDDParts[2]
			break

		case name == "PowerCapSetting" && FQDD == "System.Chassis.1":
			value, ok = value.(string)
			if !ok {
				logger.Error("power cap is not a string")
				return
			}
			updatePowerCapFlag(value.(string), &powercap_enabled)
			return

		case name == "PowerCapValue" && FQDD == "System.Chassis.1":
			key = "PowerLimit/LimitInWatts"
			var powerlimit float64
			if powercap_enabled {
				powerlimit, ok = data.Value.(float64)
				if !ok {
					logger.Error(fmt.Sprintf("power limit is %T\n", data.Value))
					return
				}
			}

			value = powerlimit
			URI = "/redfish/v1/Chassis/System.Chassis.1/Power/PowerControl"
			break

		case name == "ChassisPowerStatus" && FQDD == "System.Chassis.1":
			key = "PowerState"
			value, ok = value.(string)
			if !ok {
				logger.Error("power state is not a string")
				return
			}
			value = map_power_state(value.(string))
			URI = "/redfish/v1/Chassis/" + FQDD
			break
		default:
			return

		}

		uuid, ok := d.GetAggregateIDOK(URI)
		if !ok {
			logger.Error("Attribute not present at URI for attribute update", "FQDD", FQDD, "URI", URI)
			return
		}

		d.CommandHandler.HandleCommand(context.Background(),
			&domain.UpdateRedfishResourceProperties2{
				ID: uuid,
				Properties: map[string]interface{}{
					key: value,
				},
			})
	})

	am3Svc.AddEventHandler("IomCapabilityFn", dm_event.IomCapability, func(event eh.Event) {
		data, ok := event.Data().(*dm_event.IomCapabilityData)
		if !ok {
			logger.Error("Iom Capability event did not have Iom capability event data", "type", event.EventType, "data", event.Data())
			return
		}

		FQDD := data.Name
		if !strings.Contains(FQDD, "IOM.Slot.") {
			return
		}
		URI := "/redfish/v1/Chassis/" + FQDD + "/IOMConfiguration"
		uuid, ok := d.GetAggregateIDOK(URI)
		if !ok {
			logger.Error("Got am iomcapability event object for something that doesn't appear in our redfish tree", "FQDD", FQDD, "Calculated URI", URI)
			return
		}

		capabilities, ok := data.Capabilities.([]interface{})
		if !ok {
			logger.Error("Iom Capabilities are not a slice of interfaces", "capabilties", data.Capabilities)
			return
		}

		config_objects, ok := data.IOMConfig_objects.(map[string]interface{})
		if !ok {
			logger.Error("Iom Config Objects  are not a map of string-interfaces", "iomconfig_objects", data.IOMConfig_objects)
			return
		}

		var capabilities_copy []interface{}
		copy(capabilities_copy, capabilities)

		config_objects_copy := make(map[string]interface{})
		for key, value := range config_objects {
			config_objects_copy[key] = value
		}

		rrp_config_objects := &domain.RedfishResourceProperty{Value: config_objects_copy}
		rrp_config_objects.Parse(config_objects_copy)
		rrp_capabilities := &domain.RedfishResourceProperty{Value: capabilities_copy}
		rrp_capabilities.Parse(capabilities_copy)
		d.CommandHandler.HandleCommand(context.Background(),
			&domain.UpdateRedfishResourceProperties2{
				ID: uuid,
				Properties: map[string]interface{}{
					"Id":                       FQDD,
					"internal_mgmt_supported":  data.Internal_mgmt_supported,
					"IOMConfig_objects":        rrp_config_objects,
					"Capabilities":             rrp_capabilities,
					"Capabilities@odata.count": data.CapabilitiesCount,
				},
			})

	})

	am3Svc.AddEventHandler("FileReadEventFn", dm_event.FileReadEvent, func(event eh.Event) {
		data, ok := event.Data().(*dm_event.FileReadEventData)
		if !ok {
			logger.Error("File Read Event Data did not have event data", "type", event.EventType, "data", event.Data())
			return
		}
		key := ""
		FQDD := data.FQDD
		URI := "/redfish/v1/Managers/" + FQDD + "/CertificateService"
		switch {
		case !strings.Contains(data.FQDD, "CMC.Integrated"):
			return
		case strings.Contains(data.URI, "FactoryIdentity"):
			// Certificate is read directly.
			return
		case data.URI == "CertificateInventory":
			key = "CertificateSigningRequest"
		default:
			logger.Debug("FileReadEvent data does not meet filter criteria", "URI", data.URI)
			return
		}
		uuid, ok := d.GetAggregateIDOK(URI)
		if !ok {
			logger.Error("aggregate does not exist at URI to update", "URI", URI)
			return
		}

		var val interface{} = data.Content
		val, ok = empty_to_null(val)
		if !ok {
			logger.Error("Converting to NULL failed", "URI", URI)
			return
		}

		d.CommandHandler.HandleCommand(context.Background(),
			&domain.UpdateRedfishResourceProperties2{
				ID: uuid,
				Properties: map[string]interface{}{
					key: val,
				},
			})

	})

	am3Svc.AddEventHandler("ThermalSensorEventFn", dm_event.ThermalSensorEvent, func(event eh.Event) {
		data, ok := event.Data().(*dm_event.ThermalSensorEventData)
		if !ok {
			logger.Error("Thermal sensor event did not have thermal event data", "type", event.EventType, "data", event.Data())
			return
		}
		FQDD := data.ObjectHeader.FQDD
		sensorUri := "/redfish/v1/Chassis/System.Chassis.1/Sensors/Temperatures/" + FQDD

		// create the sensor properties, the temperatures are set to nil to start, values that are not
		// -128 are left nil.
		var sensorProperties = map[string]interface{}{
			"Name":                      data.OffsetDeviceName,
			"Description":               "Represents the properties for Temperature and Cooling",
			"LowerThresholdCritical":    nil,
			"LowerThresholdNonCritical": nil,
			"MemberId":                  data.OffsetDeviceFQDD,
			"ReadingCelsius":            nil,
			"Status": map[string]interface{}{
				"HealthRollup": health_map(data.SensorHealth),
				"State":        nil, //hardcoded
				"Health":       health_map(data.SensorHealth),
			},

			"Status/HealthRollup":       health_map(data.SensorHealth),
			"Status/State":              nil, //hardcoded
			"Status/Health":             health_map(data.SensorHealth),
			"UpperThresholdCritical":    nil,
			"UpperThresholdNonCritical": nil,
		}
		// update temperatures.
		updateTemperature(sensorProperties, "ReadingCelsius", data.SensorReading)
		updateTemperature(sensorProperties, "LowerThresholdCritical", data.LowerCriticalThreshold)
		updateTemperature(sensorProperties, "LowerThresholdNonCritical", data.LowerWarningThreshold)
		updateTemperature(sensorProperties, "UpperThresholdCritical", data.UpperCriticalThreshold)
		updateTemperature(sensorProperties, "UpperThresholdNonCritical", data.UpperWarningThreshold)

		// remove any existing one
		id, ok := d.GetAggregateIDOK(sensorUri)
		if ok && !((data.SensorStateMask & 1) == 1) {
			// exists and needs to be removed
			logger.Debug("remove sensor", "id", id, "ok", ok, "URI", sensorUri)
			d.CommandHandler.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: id})
		} else if !ok && ((data.SensorStateMask & 1) == 1) {
			// doesn't exist but neeeds to be added
			uuid := eh.NewUUID()
			logger.Debug("Need to add a sensor", "id", id, "ok", ok, "uuid", uuid, "URI", sensorUri)
			d.CommandHandler.HandleCommand(
				context.Background(),
				&domain.CreateRedfishResource{
					ID:          uuid,
					ResourceURI: sensorUri,
					Type:        "#Thermal.v1_0_0.Temperature",
					Context:     "/redfish/v1/$metadata#Thermal.Thermal",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: sensorProperties,
				},
			)
		} else if ok && ((data.SensorStateMask & 1) == 1) {
			// exists and needs to be updated
			logger.Debug("update sensor", "id", id, "URI", sensorUri)

			// only update the values from the sensor event, the rest can stay (they won't change)
			d.CommandHandler.HandleCommand(
				context.Background(),
				&domain.UpdateRedfishResourceProperties{
					ID:         id,
					Properties: sensorProperties,
				},
			)
		}
	})

	am3Svc.AddEventHandler("healthEventHandler", dm_event.HealthEvent, func(event eh.Event) {
		blackList := []string{"Group.1", "IOM", "SledSystem"}
		urlList := [][]string{}
		dmobj, ok := event.Data().(*dm_event.HealthEventData)
		if !ok {
			logger.Error("HealthEvent did not have Health event", "type", event.EventType, "data", event.Data())
			return
		}

		FQDD, _ := fqdd_attribute_name(dmobj.FQDD)

		for _, bl := range blackList {
			if strings.EqualFold(bl, FQDD.(string)) {
				return
			}
		}

		switch {
		case strings.HasPrefix(FQDD.(string), "System.Modular"),
			strings.HasPrefix(FQDD.(string), "IOM.Slot"):
			tmpUrl := [][]string{{"/redfish/v1/Chassis/" + FQDD.(string), "Status/HealthRollup"},
				{"/redfish/v1/Chassis/" + FQDD.(string), "Status/Health"},
				{"/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth", FQDD.(string) + "/Status/HealthRollup"}}
			urlList = append(urlList, tmpUrl...)

		case strings.HasPrefix(FQDD.(string), "CMC.Integrated"):
			tmpUrl := [][]string{{"/redfish/v1/Chassis/" + FQDD.(string), "Status/HealthRollup"},
				{"/redfish/v1/Chassis/" + FQDD.(string), "Status/Health"},
				{"/redfish/v1/Managers/" + FQDD.(string), "Status/HealthRollup"},
				{"/redfish/v1/Managers/" + FQDD.(string), "Status/Health"},
				{"/redfish/v1/Managers/" + FQDD.(string) + "/Redundancy", "Status/HealthRollup"},
				{"/redfish/v1/Managers/" + FQDD.(string) + "/Redundancy", "Status/Health"}}
			urlList = append(urlList, tmpUrl...)

		case strings.HasPrefix(FQDD.(string), "PowerSupply"):
			tmpUrl := [][]string{{"/redfish/v1/Chassis/System.Chassis.1/Power", "Oem/Dell/PowerSuppliesSummary/Status/HealthRollup"},
				{"/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth", FQDD.(string) + "/Status/HealthRollup"}}
			urlList = append(urlList, tmpUrl...)

		case strings.HasPrefix(FQDD.(string), "Root"):
			tmpUrl := [][]string{{"/redfish/v1/Chassis/System.Chassis.1", "Status/HealthRollup"},
				{"/redfish/v1/Chassis/System.Chassis.1", "Status/Health"}}
			urlList = append(urlList, tmpUrl...)

		case strings.HasPrefix(FQDD.(string), "Fan"):
			tmpUrl := [][]string{{"/redfish/v1/Chassis/System.Chassis.1/Thermal", "Oem/EID_674/FansSummary/Status/HealthRollup"},
				{"/redfish/v1/Chassis/System.Chassis.1/Thermal", "Oem/EID_674/FansSummary/Status/Health"},
				{"/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth", FQDD.(string) + "/Status/HealthRollup"}}
			urlList = append(urlList, tmpUrl...)

		case strings.HasPrefix(FQDD.(string), "Temperature"):
			tmpUrl := [][]string{{"/redfish/v1/Chassis/System.Chassis.1/Thermal", "Oem/EID_674/TemperaturesSummary/Status/HealthRollup"},
				{"/redfish/v1/Chassis/System.Chassis.1/Thermal", "Oem/EID_674/TemperaturesSummary/Status/Health"},
				{"/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth", FQDD.(string) + "/Status/HealthRollup"}}
			urlList = append(urlList, tmpUrl...)

		default:
			tmpUrl := [][]string{{"/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth", FQDD.(string) + "/Status/HealthRollup"}}
			urlList = append(urlList, tmpUrl...)
		}

		for _, URI := range urlList {
			uuid, ok := d.GetAggregateIDOK(URI[0])
			if !ok {
				logger.Error("healthEventHandler: URI not found", "URI", URI[0])
				continue
			}

			value, _ := empty_to_null(dmobj.Health)

			d.CommandHandler.HandleCommand(context.Background(),
				&domain.UpdateRedfishResourceProperties2{
					ID: uuid,
					Properties: map[string]interface{}{
						URI[1]: value,
					},
				})

		}
	})

	fault_lim := 10
	var tombstones []string
	fault_collection_uri := "/redfish/v1/Managers/CMC.Integrated.1/Logs/FaultList"

	am3Svc.AddEventHandler("FaultEntryAddFn", FaultEntryAdd, func(event eh.Event) {
		data, ok := event.Data().(*FaultEntryAddData)
		if !ok {
			logger.Error("FaultEntryAdd event did not match", "type", event.EventType, "data", event.Data())
			return
		}

		// check if fault remove event is already received. Can return
		i := in_array_index(data.Name, tombstones)

		if i != -1 {
			fl := len(tombstones) - 1
			for n := len(tombstones) - 1; n > i && n != 0; n -= 1 {
				tombstones[n-1] = tombstones[n]
			}
			tombstones[fl] = ""
			tombstones = tombstones[:fl]
			return
		}

		timeF, err := strconv.ParseFloat(data.Created, 64)
		if err != nil {
			logger.Debug("Mapper configuration error: Time information can not be parsed", "time", data.Created, "err", err, "set time to", 0)
			timeF = 0
		}
		createdTime := time.Unix(int64(timeF), 0)
		cTime := createdTime.Format("2006-01-02T15:04:05-07:00")

		uuid := eh.NewUUID()
		uri := fmt.Sprintf("%s/%s", fault_collection_uri, data.Name)

		// when mchars is restarted, it clears faults and expects old faults to be recreated.
		// skip re-creating old faults if this happens.
		aggID, ok := d.GetAggregateIDOK(uri)
		if ok {
			logger.Info("URI already exists, skipping add log", "aggID", aggID, "uri", uri)
			// not returning error because that will unnecessarily freak out govaluate when there really isn't an error we care about at that level
			return
		}

		d.CommandHandler.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          uuid,
				ResourceURI: uri,
				Type:        "#LogEntry.LogEntry",
				Plugin:      "ECFault",
				Context:     "/redfish/v1/$metadata#LogEntry.LogEntry",
				Headers: map[string]string{
					"Location": uri,
				},
				Privileges: map[string]interface{}{
					"GET":    []string{"Login"},
					"DELETE": []string{"ConfigureManager"},
				},
				Properties: map[string]interface{}{
					"Created":                 cTime,
					"Description":             "FaultList Entry " + data.FQDD,
					"Name":                    "FaultList Entry " + data.FQDD,
					"EntryType":               data.EntryType,
					"Id":                      data.Name,
					"MessageArgs":             data.MessageArgs,
					"MessageArgs@odata.count": len(data.MessageArgs),
					"Message":                 data.Message,
					"MessageId":               data.MessageID,
					"Category":                data.Category,
					"Oem": map[string]interface{}{
						"Dell": map[string]interface{}{
							"@odata.type": "#DellLogEntry.v1_0_0.LogEntrySummary",
							"FQDD":        data.FQDD,
							"SubSystem":   data.SubSystem,
						}},
					"OemRecordFormat": "Dell",
					"Severity":        data.Severity,
					"Action":          data.Action,
					"Links":           map[string]interface{}{},
				}})

	})

	am3Svc.AddEventHandler("FaultEntryRemoveFn", FaultEntryRemove, func(event eh.Event) {
		data, ok := event.Data().(*FaultEntryRmData)
		if !ok {
			logger.Error("FaultEntryRemove event did not match", "type", event.EventType, "data", event.Data())
			return
		}

		uri := fmt.Sprintf("%s/%s", fault_collection_uri, data.Name)

		id, ok := d.GetAggregateIDOK(uri)
		if ok {
			d.CommandHandler.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: id})
		} else {
			if len(tombstones) == fault_lim {
				tombstones = tombstones[1:]
			}

			if !in_array(data.Name, tombstones) {
				tombstones = append(tombstones, data.Name)
			}
		}

	})

	am3Svc.AddEventHandler("FaultEntriesClearFn", FaultEntriesClear, func(event eh.Event) {
		logger.Debug("Clearing all uris within base_uri", "base_uri", fault_collection_uri)

		go func() {
			uriList := d.FindMatchingURIs(func(uri string) bool { return path.Dir(uri) == fault_collection_uri })
			for _, uri := range uriList {
				id, ok := d.GetAggregateIDOK(uri)
				if ok {
					ev := ev.NewSyncEvent(domain.RedfishResourceRemoved, &domain.RedfishResourceRemovedData{
						ID:          id,
						ResourceURI: uri,
					}, time.Now())
					ev.Add(1)
					d.EventBus.PublishEvent(context.Background(), ev)
					ev.Wait()
				}
			}
		}()
	})

	am3Svc.AddEventHandler("LogEventFn", LogEvent, func(event eh.Event) {
		data, ok := event.Data().(*LogEventData)
		if !ok {
			logger.Error("LogEvent did not match", "type", event.EventType, "data", event.Data())
			return
		}

		operation := data.LogAlert
		collection_uri := "/redfish/v1/Managers/CMC.Integrated.1/Logs/Lclog"
		MAX_LOGS := 3000

		if strings.Contains(operation, "log") {
			uuid := eh.NewUUID()
			uri := fmt.Sprintf("%s/%d", collection_uri, data.Id)

			aggID, ok := d.GetAggregateIDOK(uri)
			if ok {
				logger.Debug("URI already exists, skipping add log", "aggID", aggID, "uri", uri)

			} else {
				logger.Debug("URI is new and can be added", "URI", uri)

				timeF, err := strconv.ParseFloat(data.Created, 64)
				if err != nil {
					logger.Debug("LCLOG: Time information can not be parsed", "time", data.Created, "err", err, "set time to", 0)
					timeF = 0
				}
				createdTime := time.Unix(int64(timeF), 0)
				cTime := createdTime.Format("2006-01-02T15:04:05-07:00")

				severity := data.Severity
				if data.Severity == "Informational" {
					severity = "OK"
				}

				d.CommandHandler.HandleCommand(
					context.Background(),
					&domain.CreateRedfishResource{
						ID:          uuid,
						ResourceURI: uri,
						Type:        "#LogEntry.v1_0_2.LogEntry",
						Context:     "/redfish/v1/$metadata#LogEntry.LogEntry",
						Privileges: map[string]interface{}{
							"GET": []string{"Login"},
						},
						Properties: map[string]interface{}{
							"Created":     cTime,
							"Description": data.Name,
							"Name":        data.Name,
							"EntryType":   data.EntryType,
							"Id":          data.Id,
							"Links": map[string]interface{}{
								"OriginOfCondition": map[string]interface{}{
									"@odata.id": link_mapper(data.FQDD),
								},
							},
							"MessageArgs@odata.count": len(data.MessageArgs),
							"MessageArgs":             data.MessageArgs,
							"Message":                 data.Message,
							"MessageId":               data.MessageID,
							"Oem": map[string]interface{}{
								"Dell": map[string]interface{}{
									"@odata.type": "#DellLogEntry.v1_0_0.LogEntrySummary",
									"Category":    data.Category,
									"FQDD":        data.FQDD,
								}},
							"OemRecordFormat": "Dell",
							"Severity":        severity,
							"Action":          data.Action,
						}})

				uriList := d.FindMatchingURIs(func(uri string) bool { return path.Dir(uri) == collection_uri })
				if len(uriList) > MAX_LOGS {
					// dont need to sort it until we know we are too long
					sort.Slice(uriList, func(i, j int) bool {
						idx_i, _ := strconv.Atoi(path.Base(uriList[i]))
						idx_j, _ := strconv.Atoi(path.Base(uriList[j]))
						return idx_i > idx_j
					})

					logger.Debug("too many logs, trimming", "len", len(uriList))
					go func(uriList []string) {
						for _, uri := range uriList {
							id, ok := d.GetAggregateIDOK(uri)
							if ok {
								ev := ev.NewSyncEvent(domain.RedfishResourceRemoved, &domain.RedfishResourceRemovedData{
									ID:          id,
									ResourceURI: uri,
								}, time.Now())
								ev.Add(1)
								d.EventBus.PublishEvent(context.Background(), ev)
								ev.Wait()
							}
						}
					}(uriList[MAX_LOGS:])
				}
			}
		}

		if strings.Contains(operation, "alert") {
			timeF, err := strconv.ParseFloat(data.Created, 64)
			if err != nil {
				logger.Debug("Mapper configuration error: Time information can not be parsed", "time", data.Created, "err", err, "set time to", 0)
				timeF = 0
			}
			createdTime := time.Unix(int64(timeF), 0)
			cTime := createdTime.Format("2006-01-02T15:04:05-07:00")

			//Create Alert type event:

			d.EventBus.PublishEvent(context.Background(),
				eh.NewEvent(eventservice.RedfishEvent, &eventservice.RedfishEventData{
					EventType:      "Alert",
					EventId:        data.EventId,
					EventTimestamp: cTime,
					Severity:       data.Severity,
					Message:        data.Message,
					MessageId:      data.MessageID,
					MessageArgs:    data.MessageArgs,
					//TODO MSM BUG: OriginOfCondition for events has to be a string or will be rejected
					OriginOfCondition: data.FQDD,
				}, time.Now()))
		}

		if strings.Contains(operation, "clear") {
			logger.Debug("Clearing all uris within base_uri", "base_uri", collection_uri)

			go func() {
				uriList := d.FindMatchingURIs(func(uri string) bool { return path.Dir(uri) == collection_uri })
				for _, uri := range uriList {
					id, ok := d.GetAggregateIDOK(uri)
					if ok {
						ev := ev.NewSyncEvent(domain.RedfishResourceRemoved, &domain.RedfishResourceRemovedData{
							ID:          id,
							ResourceURI: uri,
						}, time.Now())
						ev.Add(1)
						d.EventBus.PublishEvent(context.Background(), ev)
						ev.Wait()
					}
				}
			}()
		}

	})

	/*attributeLinks = []AttributeLink {
	    {FQDD: `PSU\.Slot\.\d+`,
	     FullURI: "/redfish/v1/Chassis/System.Chassis.1/Power/PowerSupplies",
	     CollectionURI: "/redfish/v1/Chassis/System.Chassis.1/Power",
	     Property: "PowerSupplies",
	    },
	  }

	  am3Svc.AddEventHandler("AttributeUpdatedFn", attributedef.AttributeUpdated, func(event eh.Event) {
	    data, ok := event.Data().(*attributedef.AttributeUpdatedData)
	    if !ok {
	      logger.Error("Attribute Updated event did not match", "type", event.EventType, "data", event.Data())
	      return
	    }

	    fqdd := data.FQDD
	    for _, a := range(attributeLinks) {
	      re := regexp.MustCompile(a.FQDD)
	      if !re.Match([]byte(fqdd)) {
	        continue
	      }

	      collection_uuid, ok := d.GetAggregateIDOK(a.CollectionURI)
	      if !ok {
	        logger.Error("AttributedUpdatedFn: Collection URI not found", "URI", a.CollectionURI)
	        continue
	      }

	      fullURI := a.FullURI+"/"+fqdd
	      updated_uuid, ok := d.GetAggregateIDOK(fullURI)
	      if !ok {
	        logger.Error("AttributedUpdatedFn: URI not found", "URI", fullURI)
	        continue
	      }
	      agg, _ := d.AggregateStore.Load(ctx, domain.AggregateType, updated_uuid)
	      redfishResource, ok := agg.(*domain.RedfishResourceAggregate)
	      if !ok {
	        logger.Error("wrong aggregate type returned")
	        continue
	      }
	      results := domain.Flatten(&redfishResource.Properties, false)
	      targetMap := results.(map[string]interface{})

	    }
	  })*/

	collections = []Collection{
		/*
			    {Prefix: "/redfish/v1/Chassis/System.Chassis.1/Sensors/Temperatures",
					  URI: "/redfish/v1/Chassis/System.Chassis.1/Thermal",
					  Property: "Temperatures",
					  Format: "expand"},

					{Prefix: "/redfish/v1/Chassis/System.Chassis.1/Sensors/Fans",
					  URI: "/redfish/v1/Chassis/System.Chassis.1/Thermal",
					  Property: "Fans",
					  Format: "expand"},

					{Prefix: "/redfish/v1/Chassis/System.Chassis.1/PowerControl",
					  URI: "/redfish/v1/Chassis/System.Chassis.1/Power",
					  Property: "PowerControl",
					  Format: "expand",
					  SpecialCase: "equals"}, //Special case

					{Prefix: "/redfish/v1/Chassis/System.Chassis.1/Power/PowerSupplies",
					  URI: "/redfish/v1/Chassis/System.Chassis.1/Power",
					  Property: "PowerSupplies",
					  Format: "expand"},

					{Prefix: "/redfish/v1/Chassis/System.Chassis.1/Power/PowerTrends-1",
					  URI: "/redfish/v1/Chassis/System.Chassis.1/Power",
					  Property: "Oem/Dell/PowerTrends",
					  Format: "expand"},

					{Prefix: "/redfish/v1/Chassis/System.Chassis.1/Power/PowerTrends-1",
					  URI: "/redfish/v1/Chassis/System.Chassis.1/Power/PowerTrends-1",
					  Property: "histograms",
					  Format: "expand"},
		*/

		//{Prefix: "/redfish/v1/Managers/CMC.Integrated.1/Logs/Lclog",
		//	URI:         "/redfish/v1/Managers/CMC.Integrated.1/Logs/Lclog",
		//	Property:    "Members",
		//	Format:      "expand",
		//	SpecialCase: "prepend"},

		//{Prefix: "/redfish/v1/Managers/CMC.Integrated.1/Logs/FaultList",
		//	URI:      "/redfish/v1/Managers/CMC.Integrated.1/Logs/FaultList",
		//	Property: "Members",
		//	Format:   "expand"},

		{Prefix: "/redfish/v1/Chassis",
			URI:      "/redfish/v1/Chassis/System.Chassis.1/Power/PowerControl",
			Property: "RelatedItem",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/Chassis",
			URI:      "/redfish/v1/Chassis",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/Managers",
			URI:      "/redfish/v1/Managers",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/AccountService/Accounts",
			URI:      "/redfish/v1/AccountService/Accounts",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/Systems",
			URI:      "/redfish/v1/Systems",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/AccountService/Roles",
			URI:      "/redfish/v1/AccountService/Roles",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/EventService/Subscriptions",
			URI:      "/redfish/v1/EventService/Subscriptions",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/TelemetryService/MetricReportDefinitions",
			URI:      "/redfish/v1/TelemetryService/MetricReportDefinitions",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/TelemetryService/MetricReports",
			URI:      "/redfish/v1/TelemetryService/MetricReports",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/SessionService/Sessions",
			URI:      "/redfish/v1/SessionService/Sessions",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/Registries",
			URI:      "/redfish/v1/Registries",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/Managers/CMC.Integrated.1/CertificateService/CertificateInventory",
			URI:      "/redfish/v1/Managers/CMC.Integrated.1/CertificateService/CertificateInventory",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/Managers/CMC.Integrated.2/CertificateService/CertificateInventory",
			URI:      "/redfish/v1/Managers/CMC.Integrated.2/CertificateService/CertificateInventory",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/Managers/CMC.Integrated.1/LogServices",
			URI:      "/redfish/v1/Managers/CMC.Integrated.1/LogServices",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/Managers/CMC.Integrated.2/LogServices",
			URI:      "/redfish/v1/Managers/CMC.Integrated.2/LogServices",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/UpdateService/FirmwareInventory",
			URI:      "/redfish/v1/UpdateService/FirmwareInventory",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs",
			URI:      "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/TaskService/Tasks",
			URI:      "/redfish/v1/TaskService/Tasks",
			Property: "Members",
			Format:   "formatOdataList"},

		{Prefix: "/redfish/v1/Managers/CMC.Integrated.1/Logs/FaultList'",
			URI:      "/redfish/v1/Managers/CMC.Integrated.1/Logs/FaultList'",
			Property: "Members",
			Format:   "formatOdataList"},
	}

	am3Svc.AddEventHandler("ResourceCreatedFn", domain.RedfishResourceCreated, func(event eh.Event) {
		data, ok := event.Data().(*domain.RedfishResourceCreatedData)
		if !ok {
			logger.Error("Redfish Resource Created event did not match", "type", event.EventType, "data", event.Data())
			return
		}

		created_uri := format_uri(data.ResourceURI)
		//fmt.Println("Resource Created: ", created_uri)

		for _, c := range collections {
			targetMap := map[string]interface{}{}
			parent := parent_uri(created_uri)
			if c.SpecialCase == "equals" && created_uri == c.Prefix {
				//special handling for PowerControl
				parent = created_uri
			}
			if parent != c.Prefix {
				continue
			}
			collection_uuid, ok := d.GetAggregateIDOK(c.URI)
			if !ok {
				logger.Error("ResourceCreatedFn: URI not found", "URI", c.URI)
				continue
			}

			if c.Format == "expand" {
				created_uuid, ok := d.GetAggregateIDOK(data.ResourceURI)
				if !ok {
					logger.Error("ResourceCreatedFn: Created URI not found", "URI", data.ResourceURI)
					continue
				}
				agg, _ := d.AggregateStore.Load(ctx, domain.AggregateType, created_uuid)
				redfishResource, ok := agg.(*domain.RedfishResourceAggregate)
				if !ok {
					logger.Error("wrong aggregate type returned")
					continue
				}

				results := domain.Flatten(&redfishResource.Properties, false)
				targetMap = results.(map[string]interface{})
			}

			if c.SpecialCase == "prepend" {
				c.Format += "_prepend"
			}

			d.CommandHandler.HandleCommand(context.Background(),
				&domain.UpdateRedfishResourceCollection{
					ID: collection_uuid,
					Properties: map[string]interface{}{
						c.Property:                  created_uri,
						c.Property + "@odata.count": 1,
					},
					Format:    c.Format,
					TargetMap: targetMap,
				})
		}

	})

	am3Svc.AddEventHandler("ResourceRemovedFn", domain.RedfishResourceRemoved, func(event eh.Event) {
		data, ok := event.Data().(*domain.RedfishResourceRemovedData)
		if !ok {
			logger.Error("Redfish Resource Removed event did not match", "type", event.EventType, "data", event.Data())
			return
		}

		removed_uri := format_uri(data.ResourceURI)
		//fmt.Println("Resource Removed: ", removed_uri)

		for _, c := range collections {
			parent := parent_uri(removed_uri)
			if parent != c.Prefix {
				continue
			}
			collection_uuid, ok := d.GetAggregateIDOK(c.URI)
			if !ok {
				logger.Error("URI not found", "URI", c.URI)
				return
			}
			d.CommandHandler.HandleCommand(context.Background(),
				&domain.UpdateRedfishResourceCollection{
					ID: collection_uuid,
					Properties: map[string]interface{}{
						c.Property:                  removed_uri,
						c.Property + "@odata.count": -1,
					},
					Format:    "remove",
					TargetMap: map[string]interface{}{},
				})
		}
	})

	powerRE := regexp.MustCompile(`PowerSupply\.(\w+)`)
	am3Svc.AddEventHandler("PowerSupplyEventFn", dm_event.PowerSupplyObjEvent, func(event eh.Event) {
		data, ok := event.Data().(*dm_event.PowerSupplyObjEventData)
		if !ok {
			logger.Error("Power Supply Event Data did not have a power supply event", "type", event.EventType, "data", event.Data())
			return
		}

		var inputvolts interface{}
		var inputcurrent interface{}
		var health interface{}

		//FQDD = System.Chassis.1#PowerSupply.4
		//URI1 = System.Chassis.1/Power/PowerSupplies/PSU.Slot.4
		//URI2 = System.Chassis.1/Sensors/PowerSupplies/PSU.Slot.4
		FQDD := data.ObjectHeader.FQDD
		matches := powerRE.FindSubmatch([]byte(FQDD))
		if len(matches) == 0 {
			logger.Error("Power Supply Event Data FQDD did not match System.Chassis.1#PowerSupply.### format", "FQDD", FQDD)
			return
		}
		PSU_slot := string(matches[len(matches)-1])

		if data.CurrentInputVolts != 0 {
			inputvolts = data.CurrentInputVolts
		} else {
			inputvolts = nil
		}
		if data.InstAmps != 0 { //limit to 2 decimal places
			inputcurrent = data.InstAmps
		} else {
			inputcurrent = nil
		}
		switch data.ObjectHeader.ObjStatus {
		case 2:
			health = "OK"
		case 3:
			health = "Warning"
		case 4:
			health = "Critical"
		default:
			health = nil
		}

		sensors_URI := "/redfish/v1/Chassis/System.Chassis.1/Sensors/PowerSupplies/PSU.Slot." + PSU_slot
		sensors_uuid, ok := d.GetAggregateIDOK(sensors_URI)
		if !ok {
			logger.Error("PowerSupplyEventFn: URI not found", "URI", sensors_URI)
		} else {
			d.CommandHandler.HandleCommand(context.Background(),
				&domain.UpdateRedfishResourceProperties2{
					ID: sensors_uuid,
					Properties: map[string]interface{}{
						"LineInputVoltage":      inputvolts,
						"Oem/Dell/InputCurrent": inputcurrent,
						"Status/HealthRollup":   health,
						"Status/Health":         health,
					},
				})
		}

		power_URI := "/redfish/v1/Chassis/System.Chassis.1/Power/PowerSupplies/PSU.Slot." + PSU_slot
		power_uuid, ok := d.GetAggregateIDOK(power_URI)
		if !ok {
			logger.Error("PowerSupplyEventFn: URI not found", "URI", power_URI)
		} else {
			d.CommandHandler.HandleCommand(context.Background(),
				&domain.UpdateRedfishResourceProperties2{
					ID: power_uuid,
					Properties: map[string]interface{}{
						"LineInputVoltage":      inputvolts,
						"Oem/Dell/InputCurrent": inputcurrent,
						"Status/HealthRollup":   health,
						"Status/Health":         health,
					},
				})
		}

	})
}

// TODO: these need to be moved into a common  location (they don't really belong here)
func fqdd_attribute_name(value interface{}) (interface{}, bool) {
	FQDD, ok := value.(string)
	if !ok {
		return nil, ok
	}

	FQDDL := strings.Split(FQDD, "#")
	if len(FQDDL) == 1 {
		return FQDDL[0], true
	}
	NAME := FQDDL[len(FQDDL)-1]
	return NAME, true
}

func empty_to_null(value interface{}) (interface{}, bool) {
	if value == "" {
		return nil, true
	}
	return value, true
}

func epoch2Date(date int64) string {
	return time.Unix(date, 0).String()
}

func traverse_struct(s interface{}, n string) interface{} {
	r := reflect.ValueOf(s)
	s = reflect.Indirect(r).FieldByName(n).Interface()

	// have to return float64 for all numeric types
	switch t := s.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
		return float64(reflect.ValueOf(t).Int())
	case float32, float64:
		return float64(reflect.ValueOf(t).Float())
	default:
		return s
	}
}

func interface_to_date(arg interface{}) interface{} {
	return time.Unix(int64(arg.(float64)), 0).Format(time.RFC3339)
}

func format_uri(resource_uri string) string {
	split := strings.Split(resource_uri, "/")
	right := split[len(split)-1]
	formatted_uri := strings.Replace(resource_uri, right, url.QueryEscape(right), 1)
	return formatted_uri
}

func parent_uri(resource_uri string) string {
	split := strings.Split(resource_uri, "/")
	parent_uri := strings.Join(split[:len(split)-1], "/")
	return parent_uri
}

func in_array(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// Take a string to bool map and set all the values to be the given bool
func set_all_in_map_bool(in_map map[string]bool, value bool) {
	for key, _ := range in_map {
		in_map[key] = value
	}
}

func in_array_index(a string, list []string) int {
	for i, b := range list {
		if b == a {
			return i
		}
	}
	return -1
}

func isSledProfile(FQDD string, Name string) bool {
	lhs := strings.Contains(FQDD, "System.Modular.")
	rhs := (Name == "SledProfile")
	return (lhs && rhs)
}

func updatePowerCapFlag(Value string, powercap_enabled *bool) {

	if Value == "Enabled" {
		*powercap_enabled = true
	} else {
		*powercap_enabled = false
	}
}

func map_power_state(value string) string {
	switch value {
	case "Chassis Standby Power State":
		return "Off"
	case "Chassis Power On State":
		return "On"
	case "Chassis Powering On State":
		return "PoweringOn"
	case "Chassis Powering Off State":
		return "PoweringOff"
	default:
		return ""
	}
}

func isRowOrColumn(FQDD interface{}, Name interface{}, rowOrColumn interface{}) bool {
	return (FQDD == "System.Chassis.1" && Name == rowOrColumn)
}

func value_to_string(value interface{}) (interface{}, bool) {
	switch t := value.(type) {
	case uint, uint8, uint16, uint32, uint64:
		str := strconv.FormatUint(reflect.ValueOf(t).Uint(), 10)
		return str, true
	case float32, float64:
		str := strconv.FormatFloat(reflect.ValueOf(t).Float(), 'G', -1, 64)
		return str, true
	case string:
		return t, true
	case int, int8, int16, int32, int64:
		str := strconv.FormatInt(reflect.ValueOf(t).Int(), 10)
		return str, true
	default:
		return nil, false
	}
}

// Split a string and return the specified index, if it exists. (-1 for last)
func split_string_index(s string, d string, i int) string {
	split := strings.Split(s, d)

	// Verification on requested index
	if i < 0 && -i <= len(split) {
		// Negative value for i (in bounds of course) is assumed to mean to
		// traverse backwards similar to python
		i = len(split) + i
	} else if i >= len(split) || -i >= len(split) {
		// Index out of bounds will assume to mean that the last element
		// is requested. :)
		i = len(split) - 1
	}

	return split[i]
}

/*********************************** EOF ***********************************/
