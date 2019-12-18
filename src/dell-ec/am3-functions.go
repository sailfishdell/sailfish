package dell_ec

import (
	"context"
	"strings"

	eh "github.com/looplab/eventhorizon"

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

	// Start addressing AttributeUpdated Events
	//attribute_mappings := map[string][]string{}
	am3Svc.AddEventHandler("redfish_properties_linked_to_config_attributes_setup", domain.RedfishResourceCreated, func(event eh.Event) {
	})
	am3Svc.AddEventHandler("redfish_properties_linked_to_config_attributes_delete", domain.RedfishResourceRemoved, func(event eh.Event) {
	})
	am3Svc.AddEventHandler("redfish_properties_linked_to_config_attributes_update", attributedef.AttributeUpdated, func(event eh.Event) {})
}
