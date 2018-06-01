package powersupply

import (
	"context"

	"github.com/superchalupa/go-redfish/src/dell-resources/ar_mapper"
	attr_prop "github.com/superchalupa/go-redfish/src/dell-resources/attribute-property"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

// So... this class is set up in a somewhat interesting way to support having
// PSU.Slot.N objects both as PowerSupplies/PSU.Slot.N as well as in the main
// Power object.

func NewView(ctx context.Context, logger log.Logger, chasName, psuName string, s *model.Model, attributeView *view.View, c *ar_mapper.ARMappingController, d *attr_prop.ARDump, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) (*view.View, map[string]interface{}) {

	v := view.NewView(
		view.WithURI("/redfish/v1/Chassis/" + chasName + "/Power/" + psuName),
		view.WithModel("default", s),
		view.WithController("ar_mapper", c),
		view.WithController("ar_dumper", d),
		view.WithFormatter("attributeFormatter", attr_prop.FormatAttributeDump),
	)

	domain.RegisterPlugin(func() domain.Plugin { return v })

	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#Power.v1_0_2.Power",
			Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: GetViewFragment(v, attributeView, v.GetURI()),
		})

	return v, GetViewFragment(v, attributeView, v.GetURI())
}

//
// this view fragment can be attached elsewhere in the tree
//
func GetViewFragment(regularView *view.View, attributesView *view.View, uri string) map[string]interface{} {
	return map[string]interface{}{
		"@odata.type":             "#Power.v1_0_2.PowerSupply",
		"@odata.context":          "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
		"@odata.id":               uri,
		"Name@meta":               regularView.Meta(view.PropGET("name")),
		"MemberId@meta":           regularView.Meta(view.PropGET("unique_id")),
		"PowerCapacityWatts@meta": regularView.Meta(view.PropGET("capacity_watts")),
		"LineInputVoltage@meta":   regularView.Meta(view.PropGET("line_input_voltage")),
		"FirmwareVersion@meta":    regularView.Meta(view.PropGET("firmware_version")),

		"Status": map[string]interface{}{
			"HealthRollup": "OK",
			"State":        "Enabled",
			"Health":       "OK",
		},

		"Oem": map[string]interface{}{
			"Dell": map[string]interface{}{
				"@odata.type":       "#DellPower.v1_0_0.DellPowerSupply",
				"ComponentID@meta":  regularView.Meta(view.PropGET("component_id")),
				"InputCurrent@meta": regularView.Meta(view.PropGET("input_current")),
				"Attributes@meta":   attributesView.Meta(view.GETProperty("attributes"), view.GETFormatter("attributeFormatter"), view.GETModel("default"), view.PropPATCH("attributes", "ar_dump")),
			},
		},
	}
}
