package fans

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

// So... this class is set up in a somewhat interesting way to support having
// Fan.Slot.N objects both as PowerSupplies/PSU.Slot.N as well as in the main
// Power object.

func AddView(ctx context.Context, logger log.Logger, s *model.Service, attr *model.Service, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) map[string]interface{} {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          model.GetUUID(s),
			Collection:  false,
			ResourceURI: model.GetOdataID(s),
			Type:        "#Thermal.v1_0_2.Fan",
			Context:     "/redfish/v1/$metadata#Thermal.Thermal",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: GetViewFragment(s, attr),
		})

	return GetViewFragment(s, attr)
}

func GetViewFragment(s *model.Service, attr *model.Service) map[string]interface{} {
	return map[string]interface{}{
		"@odata.type":       "#Thermal.v1_0_2.Fan",
		"@odata.context":    "/redfish/v1/$metadata#Thermal.Thermal",
		"@odata.id":         model.GetOdataID(s),
		"Description":       "Represents the properties for Fan and Cooling",
		"FanName@meta":      s.Meta(model.PropGET("name")),
		"MemberId@meta":     s.Meta(model.PropGET("unique_id")),
		"ReadingUnits@meta": s.Meta(model.PropGET("reading_units")),
		"Reading@meta":      s.Meta(model.PropGET("reading")),
		"Status": map[string]interface{}{
			"HealthRollup": "OK",
			"Health":       "OK",
		},
		"Oem": map[string]interface{}{
			"ReadingUnits@meta":    s.Meta(model.PropGET("oem_reading_units")),
			"Reading@meta":         s.Meta(model.PropGET("oem_reading")),
			"FirmwareVersion@meta": s.Meta(model.PropGET("firmware_version")),
			"HardwareVersion@meta": s.Meta(model.PropGET("hardware_version")),
			"GraphicsURI@meta":     s.Meta(model.PropGET("graphics_uri")),
			"Attributes@meta":      map[string]interface{}{"GET": map[string]interface{}{"plugin": string(attr.PluginType())}},
		},
	}
}
