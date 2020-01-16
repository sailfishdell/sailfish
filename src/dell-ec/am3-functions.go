package dell_ec

import (
	"context"
	"strings"
  "regexp"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
	"github.com/superchalupa/sailfish/src/ocp/telemetryservice"
	"reflect"
	"strings"
	"time"

	"github.com/superchalupa/sailfish/godefs"
	"github.com/superchalupa/sailfish/src/dell-resources/attributedef"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func addAM3Functions(logger log.Logger, am3Svc *am3.Service, d *domain.DomainObjects) {
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
					"Oem/EnergyConsumptionStartTime": epoch2Date(data.CwStartTime),
					"Oem/EnergyConsumptionkWh":       int(data.CumulativeWatts / 1000),
					"Oem/MaxPeakWatts":               data.PeakWatts,
					"Oem/MaxPeakWattsTime":           epoch2Date(data.PwReadingTime),
					"Oem/MinPeakWatts":               data.MinWatts,
					"Oem/MinPeakWattsTime":           epoch2Date(data.MinwReadingTime),
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
			logger.Error("updateFanData did not have fan event", "type", event.EventType, "data", event.Data())
			return
		}
		FQDD := data.FQDD
		arr := strings.Split(FQDD, "#")
		// FQDD can be either IOM.Slot.X or System.Chassis.1#System.Modular.X#Power
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
				logger.Error("URI not found", "URI", URI[0])
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
    re := regexp.MustCompile(`PowerSupply\.(\w+)`)
    matches := re.FindSubmatch([]byte(FQDD))
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
      logger.Error("URI not found", "URI", sensors_URI)
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
      logger.Error("URI not found", "URI", power_URI)
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
