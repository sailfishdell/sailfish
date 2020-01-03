package dell_ec

import (
	"context"
	"strings"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"

	"github.com/superchalupa/sailfish/godefs"
	"github.com/superchalupa/sailfish/src/dell-resources/attributedef"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func addAM3Functions(logger log.Logger, am3Svc *am3.Service, d *domain.DomainObjects) {
	am3Svc.AddEventHandler("modular_update_fan_data", godefs.FanEvent, func(event eh.Event) {
		dmobj, ok := event.Data().(*godefs.DMObject)
		fanobj, ok := dmobj.Data.(*godefs.DM_thp_fan_data_object)
		if !ok {
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

    am3Svc.AddEventHandler("healthEventHandler", dm_event.HealthEvent, func(event eh.Event) {
        blackList := [] string {"Group.1", "IOM","SledSystem"}
        urlList := [][]string {}
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
            tmpUrl := [][]string {{"/redfish/v1/Chassis/" + FQDD.(string), "Status/HealthRollup"},
                                  {"/redfish/v1/Chassis/" + FQDD.(string), "Status/Health"},
                                  {"/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth", FQDD.(string) + "/Status/HealthRollup"}}
            urlList = append(urlList, tmpUrl...)

        case strings.HasPrefix(FQDD.(string), "CMC.Integrated"):
            tmpUrl := [][]string {{"/redfish/v1/Chassis/" + FQDD.(string), "Status/HealthRollup"},
                                  {"/redfish/v1/Chassis/" + FQDD.(string), "Status/Health"},
                                  {"/redfish/v1/Managers/" + FQDD.(string), "Status/HealthRollup"},
                                  {"/redfish/v1/Managers/" + FQDD.(string), "Status/Health"},
                                  {"/redfish/v1/Managers/" + FQDD.(string) + "/Redundancy", "Status/HealthRollup"},
                                  {"/redfish/v1/Managers/" + FQDD.(string) + "/Redundancy", "Status/Health"}}
            urlList = append(urlList, tmpUrl...)

        case strings.HasPrefix(FQDD.(string), "PowerSupply"):
            tmpUrl := [][]string {{"/redfish/v1/Chassis/System.Chassis.1/Power", "Oem/Dell/PowerSuppliesSummary/Status/HealthRollup"},
                                  {"/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth", FQDD.(string) + "/Status/HealthRollup"}}
            urlList = append(urlList, tmpUrl...)

        case strings.HasPrefix(FQDD.(string), "Root"):
            tmpUrl := [][]string {{"/redfish/v1/Chassis/System.Chassis.1", "Status/HealthRollup"},
                                  {"/redfish/v1/Chassis/System.Chassis.1", "Status/Health"}}
            urlList = append(urlList, tmpUrl...)

        case strings.HasPrefix(FQDD.(string), "Fan"):
            tmpUrl := [][]string {{"/redfish/v1/Chassis/System.Chassis.1/Thermal", "Oem/EID_674/FansSummary/Status/HealthRollup"},
                                  {"/redfish/v1/Chassis/System.Chassis.1/Thermal", "Oem/EID_674/FansSummary/Status/Health"},
                                  {"/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth", FQDD.(string) + "/Status/HealthRollup"}}
            urlList = append(urlList, tmpUrl...)

        case strings.HasPrefix(FQDD.(string), "Temperature"):
            tmpUrl := [][]string {{"/redfish/v1/Chassis/System.Chassis.1/Thermal", "Oem/EID_674/TemperaturesSummary/Status/HealthRollup"},
                                  {"/redfish/v1/Chassis/System.Chassis.1/Thermal", "Oem/EID_674/TemperaturesSummary/Status/Health"},
                                  {"/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth", FQDD.(string) + "/Status/HealthRollup"}}
            urlList = append(urlList, tmpUrl...)

        default:
            tmpUrl := [][]string {{"/redfish/v1/Chassis/System.Chassis.1/SubSystemHealth", FQDD.(string) + "/Status/HealthRollup"}}
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
                        URI[1] : value,
                    },
            })

        }
    })



	// Start addressing AttributeUpdated Events
	//attribute_mappings := map[string][]string{}
	am3Svc.AddEventHandler("redfish_properties_linked_to_config_attributes_setup", domain.RedfishResourceCreated, func(event eh.Event) {
	})
	am3Svc.AddEventHandler("redfish_properties_linked_to_config_attributes_delete", domain.RedfishResourceRemoved, func(event eh.Event) {
	})
	am3Svc.AddEventHandler("redfish_properties_linked_to_config_attributes_update", attributedef.AttributeUpdated, func(event eh.Event) {})
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
    NAME := FQDDL[len(FQDDL) - 1]
    return NAME, true
}

func empty_to_null(value interface{}) (interface{}, bool) {
    if value == "" {
        return nil, true
    }
    return value, true
}
