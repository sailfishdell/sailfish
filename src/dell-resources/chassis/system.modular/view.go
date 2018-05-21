package sled_chassis

import (
	"context"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func AddView(s *plugins.Service, ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          plugins.GetUUID(s),
			Collection:  false,
			ResourceURI: plugins.GetOdataID(s),
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
				"Links": map[string]interface{}{
					"ManagedBy@meta": s.Meta(plugins.PropGET("managed_by")),
				},
				// TODO: "ManagedBy@odata.count": 1

				"SKU@meta":        s.Meta(plugins.PropGET("service_tag")),
				"PowerState@meta": s.Meta(plugins.PropGET("power_state")),

				"Description":  "",
				"SerialNumber": "",
				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State":        "Enabled",
					"Health":       "OK",
				},
				"PartNumber":   "",
				"Name":         "",
				"ChassisType":  "Sled",
				"Model":        "PowerEdge MX740c",
				"Manufacturer": "",
				"Oem": map[string]interface{}{
					"OemChassis": map[string]interface{}{
						"@odata.id": "/redfish/v1/Chassis/System.Modular.1/Attributes",
					},
				},
				"Actions": map[string]interface{}{
					"Oem": map[string]interface{}{
						"#DellChassis.v1_0_0#DellChassis.PeripheralMapping": map[string]interface{}{
							"MappingType@Redfish.AllowableValues": []string{
								"Accept",
								"Clear",
							},
							"target": "/redfish/v1/Chassis/System.Modular.1/Actions/Oem/DellChassis.PeripheralMapping",
						},
						"#Chassis.VirtualReseat": map[string]interface{}{
							"target": "/redfish/v1/Chassis/System.Modular.1/Actions/Chassis.VirtualReseat",
						},
						"#DellChassis.v1_0_0.PeripheralMapping": map[string]interface{}{
							"MappingType@Redfish.AllowableValues": []string{
								"Accept",
								"Clear",
							},
							"target": "/redfish/v1/Chassis/System.Modular.1/Actions/Oem/DellChassis.PeripheralMapping",
						},
						"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
							"target": "/redfish/v1/Chassis/System.Modular.1/Actions/Oem/DellChassis.VirtualReseat",
						},
					},
				},
			}})
}
