package firmware_inventory

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"
	"sync"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("inv_view",
    func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ID:          vw.GetUUID(),
					ResourceURI: vw.GetURI(),
					Type:        "#SoftwareInventory.v1_0_0.SoftwareInventory",
					Context:     params["rooturi"].(string) + "/$metadata#SoftwareInventory.SoftwareInventory",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Description@meta": vw.Meta(view.GETProperty("fw_description"), view.GETModel("swinv")),
						"Id@meta":          vw.Meta(view.GETProperty("fw_id"), view.GETModel("firm")),
						"Name@meta":        vw.Meta(view.GETProperty("fw_name"), view.GETModel("swinv")),
						"Updateable@meta":  vw.Meta(view.GETProperty("fw_updateable"), view.GETModel("swinv")),
						"Version@meta":     vw.Meta(view.GETProperty("fw_version"), view.GETModel("swinv")),
						"Status": map[string]interface{}{
							"State":  "Enabled",
							"Health": "OK",
						},
						"Oem": map[string]interface{}{
							"EID_674": map[string]interface{}{
								"ComponentId@meta": vw.Meta(view.GETProperty("fw_device_class"), view.GETModel("swinv")),
								"InstallDate@meta": vw.Meta(view.GETProperty("fw_install_date"), view.GETModel("swinv")),

								"@odata.type":                  "#EID_674_SoftwareInventory.v1_0_0.OemSoftwareInventory",
								"FQDD@meta":                    vw.Meta(view.GETProperty("fw_fqdd_list"), view.GETModel("firm")),
								"FQDD@odata.count@meta":        vw.Meta(view.GETProperty("fw_fqdd_list_count"), view.GETModel("firm")),
								"RelatedItem@meta":             vw.Meta(view.GETProperty("fw_related_list"), view.GETModel("firm")),
								"RelatedItem@odata.count@meta": vw.Meta(view.GETProperty("fw_related_list_count"), view.GETModel("firm")),
							},
						},
					}},
			}, nil
		})

	return
}
