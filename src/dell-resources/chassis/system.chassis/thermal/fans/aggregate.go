package fans

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#Thermal.v1_0_2.Fan",
			Context:     "/redfish/v1/$metadata#Thermal.Thermal",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"@odata.type":    "#Thermal.v1_0_2.Fan",
				"@odata.context": "/redfish/v1/$metadata#Thermal.Thermal",
				"@odata.id":      v.GetURI(),
				"Description":    "Represents the properties for Fan and Cooling",
				"FanName@meta":   v.Meta(view.PropGET("name")),
				"MemberId@meta":  v.Meta(view.PropGET("unique_id")),
				"ReadingUnits":   "RPM",
				"Reading@meta":   v.Meta(view.PropGET("rpm")),
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("health")),
					"Health@meta":       v.Meta(view.PropGET("health")),
				},
				"Oem": map[string]interface{}{
					"ReadingUnits":         "Percent",
					"Reading@meta":         v.Meta(view.PropGET("Fanpwm_int")),
					"FirmwareVersion@meta": v.Meta(view.PropGET("firmware_version")),
					"HardwareVersion@meta": v.Meta(view.PropGET("hardware_version")),
					"GraphicsURI@meta":     v.Meta(view.PropGET("graphics_uri")),
					"Attributes@meta":      v.Meta(view.GETProperty("attributes"), view.GETFormatter("attributeFormatter"), view.GETModel("default"), view.PropPATCH("attributes", "ar_dump")),
				},
			},
		})
}
