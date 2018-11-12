package system_chassis

import (
	"context"
	"sync"

	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("idrac_system_chassis",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Chassis.v1_0_2.Chassis",
					Context:     params["rooturi"].(string) + "/$metadata#ChassisCollection.ChassisCollection/Members/$entity",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Id@meta":           vw.Meta(view.GETProperty("unique_name"), view.GETModel("default")),
						"SerialNumber@meta": vw.Meta(view.GETProperty("serial"), view.GETModel("default")),
						"ChassisType@meta":  vw.Meta(view.GETProperty("chassis_type"), view.GETModel("default")),
						"Model@meta":        vw.Meta(view.GETProperty("model"), view.GETModel("default")),
						"Manufacturer@meta": vw.Meta(view.GETProperty("manufacturer"), view.GETModel("default")),
						"PartNumber@meta":   vw.Meta(view.GETProperty("part_number"), view.GETModel("default")),
						"Name@meta":         vw.Meta(view.GETProperty("name"), view.GETModel("default")),
						"AssetTag@meta":     vw.Meta(view.GETProperty("asset_tag"), view.GETModel("default")),
						"Description@meta":  vw.Meta(view.GETProperty("description"), view.GETModel("default")),
						"PowerState@meta":   vw.Meta(view.GETProperty("power_state"), view.GETModel("default")),

						"IndicatorLED@meta": vw.Meta(view.GETProperty("indicator_led"), view.GETModel("default")),
						"SKU@meta":          vw.Meta(view.GETProperty("service_tag"), view.GETModel("default")),

						"Links": map[string]interface{}{
							"ManagedBy@meta":             vw.Meta(view.GETProperty("managed_by"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"ManagedBy@odata.count@meta": vw.Meta(view.GETProperty("managed_by"), view.GETFormatter("count"), view.GETModel("default")),
						},

						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health")), //smil call?
							"State":             "Enabled",                       //hardcoded
							"Health@meta":       vw.Meta(view.PropGET("health")), //smil call?
						},

						"Power":   map[string]interface{}{"@odata.id": vw.GetURI() + "/Power"},
						"Thermal": map[string]interface{}{"@odata.id": vw.GetURI() + "/Thermal"},
						"Oem": map[string]interface{}{
							"Dell": map[string]interface{}{
								"SubSystemHealth": map[string]interface{}{
									"@odata.id": vw.GetURI() + "/SubSystemHealth",
								},
								"Slots": map[string]interface{}{
									"@odata.id": vw.GetURI() + "/Slots",
								},
								"SlotConfigs": map[string]interface{}{
									"@odata.id": vw.GetURI() + "/SlotConfigs",
								},
								"OemChassis": map[string]interface{}{
									"@odata.id": vw.GetURI() + "/Attributes",
								},
							},
						},

						"Actions": map[string]interface{}{
							"#Chassis.Reset": map[string]interface{}{
								"ResetType@Redfish.AllowableValues": []string{
									"On",
									"ForceOff",
									"GracefulShutdown",
									"GracefulRestart",
								},
								"target": vw.GetActionURI("chassis.reset"),
							},
							"Oem": map[string]interface{}{
								"#MSMConfigBackupURI": map[string]interface{}{
									"target": vw.GetActionURI("msmconfigbackup"),
								},
								"#DellChassis.v1_0_0.MSMConfigBackup": map[string]interface{}{
									"target": vw.GetActionURI("chassis.msmconfigbackup"),
								},
							},
						},
					}}}, nil
		})
}
