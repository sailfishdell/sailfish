package update_service

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"

	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, root *view.View, v *view.View, ch eh.CommandHandler) *view.View {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),

			Type:    "#SoftwareInventory.v1_0_0.SoftwareInventory",
			Context: root.GetURI() + "/$metadata#SoftwareInventory.SoftwareInventory",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":          "Installed-104850-00.35.6A",
				"Name":        "PSU Firmware",
				"Updateable":  true,
				"Version":     "00.35.6A",
				"Description": "Represents Firmware Inventory",
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"Oem": map[string]interface{}{
					"EID_674": map[string]interface{}{
						"@odata.type": "#EID_674_SoftwareInventory.v1_0_0.OemSoftwareInventory",
						"InstallDate": null,
						//"RelatedItem@odata.count": 4,
						//"FQDD@odata.count": 4,
						"FQDD": []string{
							"PSU.Slot.1",
							"PSU.Slot.2",
							"PSU.Slot.4",
							"PSU.Slot.5",
						},
						"RelatedItem": []map[string]interface{}{
							{
								"@odata.id": "/redfish/v1/Chassis/System.Chassis.1/Power/PowerSupplies/PSU.Slot.1",
							},
							{
								"@odata.id": "/redfish/v1/Chassis/System.Chassis.1/Power/PowerSupplies/PSU.Slot.2",
							},
							{
								"@odata.id": "/redfish/v1/Chassis/System.Chassis.1/Power/PowerSupplies/PSU.Slot.4",
							},
							{
								"@odata.id": "/redfish/v1/Chassis/System.Chassis.1/Power/PowerSupplies/PSU.Slot.5",
							},
						},
						"ComponentId": "104850",
					},
				},
			}})

	return v
}
