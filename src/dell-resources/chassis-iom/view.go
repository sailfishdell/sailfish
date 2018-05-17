package iom_chassis

import (
	"context"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func (s *service) AddView(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          s.GetUUID(),
			Collection:  false,
			ResourceURI: s.GetOdataID(),
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
				"Id":             s.GetUniqueName(),
				"ManagedBy@meta": s.Meta(plugins.PropGET("managed_by")),
	             // TODO: "ManagedBy@odata.count": 1

				"SKU":          "",
				"PowerState":   "On",
				"Description":  "PowerEdge MX5000s SAS Switch",
				"AssetTag":     "",
				"SerialNumber": "CNFCP007BH000S",
				"PartNumber":   "0PG5NRX30",
				"Name":         "PowerEdge MX5000s SAS",
				"ChassisType":  "Module",
				"IndicatorLED": "Lit",
				"Model":        "PowerEdge MX5000s SAS",
				"Manufacturer": "Dell EMC",
				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State":        "Enabled",
					"Health":       "OK",
				},
				"Oem": map[string]interface{}{
					"Dell": map[string]interface{}{
						"ServiceTag":           "",
						"InstPowerConsumption": 24,
						"OemChassis": map[string]interface{}{
							"@odata.id": "/redfish/v1/Chassis/" + s.GetUniqueName() + "/Attributes",
						},
						"OemIOMConfiguration": map[string]interface{}{
							"@odata.id": "/redfish/v1/Chassis/" + s.GetUniqueName() + "/IOMConfiguration",
						},
					},
				},

				"Actions": map[string]interface{}{
					"#Chassis.Reset": map[string]interface{}{
						"ResetType@Redfish.AllowableValues": []string{
							"On",
							"GracefulShutdown",
							"GracefulRestart",
						},
						"target": "/redfish/v1/Chassis/" + s.GetUniqueName() + "/Actions/Chassis.Reset",
					},
					"Oem": map[string]interface{}{
						"DellChassis.v1_0_0#DellChassis.ResetPeakPowerConsumption": map[string]interface{}{
							"target": "/redfish/v1/Chassis/" + s.GetUniqueName() + "/Actions/Oem/DellChassis.ResetPeakPowerConsumption",
						},
						"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
							"target": "/redfish/v1/Chassis/" + s.GetUniqueName() + "/Actions/Oem/DellChassis.VirtualReseat",
						},
						"#DellChassis.v1_0_0.ResetPeakPowerConsumption": map[string]interface{}{
							"target": "/redfish/v1/Chassis/" + s.GetUniqueName() + "/Actions/Oem/DellChassis.ResetPeakPowerConsumption",
						},
						"DellChassis.v1_0_0#DellChassis.VirtualReseat": map[string]interface{}{
							"target": "/redfish/v1/Chassis/" + s.GetUniqueName() + "/Actions/Oem/DellChassis.VirtualReseat",
						},
					},
				},
			}})


}
