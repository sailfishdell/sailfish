package firmware_inventory

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"

	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, root *view.View, v *view.View, ch eh.CommandHandler) *view.View {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
			Type:        "#SoftwareInventory.v1_0_0.SoftwareInventory",
			Context:     root.GetURI() + "/$metadata#SoftwareInventory.SoftwareInventory",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Description@meta": v.Meta(view.GETProperty("fw_description"), view.GETModel("swinv")),
				"Id@meta":          v.Meta(view.GETProperty("fw_id"), view.GETModel("firm")),
				"Name@meta":        v.Meta(view.GETProperty("fw_name"), view.GETModel("swinv")),
				"Updateable@meta":  v.Meta(view.GETProperty("fw_updateable"), view.GETModel("swinv")),
				"Version@meta":     v.Meta(view.GETProperty("fw_version"), view.GETModel("swinv")),
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"Oem": map[string]interface{}{
					"EID_674": map[string]interface{}{
						"ComponentId@meta": v.Meta(view.GETProperty("fw_device_class"), view.GETModel("swinv")),
						"InstallDate@meta": v.Meta(view.GETProperty("fw_install_date"), view.GETModel("swinv")),

						"@odata.type": "#EID_674_SoftwareInventory.v1_0_0.OemSoftwareInventory",
						//"RelatedItem@odata.count": 4,
						//"FQDD@odata.count": 4,
						"FQDD@meta":        v.Meta(view.GETProperty("fw_fqdd_list"), view.GETModel("firm")),
						"RelatedItem@meta": v.Meta(view.GETProperty("fw_related_list"), view.GETModel("firm")),
					},
				},
			}})

	return v
}
