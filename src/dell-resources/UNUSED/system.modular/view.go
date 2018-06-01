package sled_chassis

import (
	"context"

	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func AddView(s *model.Service, ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          model.GetUUID(s),
			Collection:  false,
			ResourceURI: model.GetOdataID(s),
			Type:        "#Chassis.v1_0_2.Chassis",
			Context:     "/redfish/v1/$metadata#ChassisCollection.ChassisCollection/Members/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id": s.GetProperty("unique_name").(string),

				"SKU@meta":          s.Meta(model.PropGET("service_tag")),
				"PowerState@meta":   s.Meta(model.PropGET("power_state")),
				"ChassisType@meta":  s.Meta(model.PropGET("chassis_type")),
				"Model@meta":        s.Meta(model.PropGET("model")),
				"Manufacturer@meta": s.Meta(model.PropGET("manufacturer")),
				"SerialNumber":      s.Meta(model.PropGET("serial")),

				"Links": map[string]interface{}{
					// TODO: "ManagedBy@odata.count": 1
					"ManagedBy@meta": s.Meta(model.PropGET("managed_by")),
				},

				"Description": "",
				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State":        "Enabled",
					"Health":       "OK",
				},
				"PartNumber": "",
				"Name":       "",
				"Oem": map[string]interface{}{
					"OemChassis": map[string]interface{}{
						"@odata.id": model.GetOdataID(s) + "/Attributes",
					},
				},
				"Actions": map[string]interface{}{
					"Oem": map[string]interface{}{
						"#DellChassis.v1_0_0#DellChassis.PeripheralMapping": map[string]interface{}{
							"MappingType@Redfish.AllowableValues": []string{
								"Accept",
								"Clear",
							},
							"target": model.GetOdataID(s) + "/Actions/Oem/DellChassis.PeripheralMapping",
						},
						"#Chassis.VirtualReseat": map[string]interface{}{
							"target": model.GetOdataID(s) + "/Actions/Chassis.VirtualReseat",
						},
						"#DellChassis.v1_0_0.PeripheralMapping": map[string]interface{}{
							"MappingType@Redfish.AllowableValues": []string{
								"Accept",
								"Clear",
							},
							"target": model.GetOdataID(s) + "/Actions/Oem/DellChassis.PeripheralMapping",
						},
						"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
							"target": model.GetOdataID(s) + "/Actions/Oem/DellChassis.VirtualReseat",
						},
					},
				},
			}})
}
