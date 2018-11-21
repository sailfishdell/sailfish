package dell_ec

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func RegisterSledAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("sled",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{

					ResourceURI: vw.GetURI(),
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
						"Id@meta":           vw.Meta(view.GETProperty("unique_name"), view.GETModel("default")),
						"SKU@meta":          vw.Meta(view.GETProperty("service_tag"), view.GETModel("default")),
						"PowerState@meta":   vw.Meta(view.GETProperty("power_state"), view.GETModel("default")),
						"ChassisType@meta":  vw.Meta(view.GETProperty("chassis_type"), view.GETModel("default")),
						"Model@meta":        vw.Meta(view.GETProperty("model"), view.GETModel("default")),
						"Manufacturer@meta": vw.Meta(view.GETProperty("manufacturer"), view.GETModel("default")),
						"SerialNumber@meta": vw.Meta(view.GETProperty("serial"), view.GETModel("default")),
						"Description@meta":  vw.Meta(view.GETProperty("description"), view.GETModel("default")),

						"Links": map[string]interface{}{
							"ManagedBy@meta":             vw.Meta(view.GETProperty("managed_by"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"ManagedBy@odata.count@meta": vw.Meta(view.GETProperty("managed_by"), view.GETFormatter("count"), view.GETModel("default")),
						},

						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health")),
							"State":             "Enabled", //hardcoded
							"Health@meta":       vw.Meta(view.PropGET("health")),
						},
						"PartNumber@meta": vw.Meta(view.GETProperty("part_number"), view.GETModel("default")),
						"Name@meta":       vw.Meta(view.GETProperty("name"), view.GETModel("default")),
						"Oem": map[string]interface{}{
							"Dell": map[string]interface{}{
								"InstPowerConsumption@meta": vw.Meta(view.PropGET("Instantaneous_Power")),
							},
							"OemChassis": map[string]interface{}{
								"@odata.id": vw.GetURI() + "/Attributes",
							},
						},
						"Actions": map[string]interface{}{
							"Oem": map[string]interface{}{
								// TODO: Remove per JIT-66996
								"#DellChassis.v1_0_0#DellChassis.PeripheralMapping": map[string]interface{}{
									"MappingType@Redfish.AllowableValues": []string{
										"Accept",
										"Clear",
									},
									"target": vw.GetActionURI("chassis.peripheralmapping"),
								},
								"#Chassis.VirtualReseat": map[string]interface{}{
									"target": vw.GetActionURI("sledvirtualreseat"),
								},
								"#DellChassis.v1_0_0.PeripheralMapping": map[string]interface{}{
									"MappingType@Redfish.AllowableValues": []string{
										"Accept",
										"Clear",
									},
									"target": vw.GetActionURI("chassis.peripheralmapping"),
								},
								"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
									"target": vw.GetActionURI("chassis.sledvirtualreseat"),
								},
							},
						},
					}}}, nil
		})

	s.RegisterAggregateFunction("system_chassis",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
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

						"IndicatorLED": vw.Meta(view.GETModel("default"), view.PropPATCH("indicator_led", "ar_mapper"), view.GETProperty("indicator_led")),
						"SKU@meta":     vw.Meta(view.GETProperty("service_tag"), view.GETModel("default")),

						"Links": map[string]interface{}{
							"ManagedBy@meta":             vw.Meta(view.GETProperty("managed_by"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"ManagedBy@odata.count@meta": vw.Meta(view.GETProperty("managed_by"), view.GETFormatter("count"), view.GETModel("default")),
						},

						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health")),
							"State":             "Enabled", //hardcoded
							"Health@meta":       vw.Meta(view.PropGET("health")),
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

	s.RegisterAggregateFunction("subsyshealth",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "DellSubSystemHealth.v1_0_0.DellSubSystemHealth",
					Context:     "/redfish/v1/$metadata#ChassisCollection.ChassisCollection/Members/$entity",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
					// The below doesn't work because it creates a property with name "".
					// So for now, the awesome mapper function will just do an update
					// resource command directly
					// "@meta": vw.Meta(view.PropGET("health")),
					}},

				&domain.UpdateRedfishResourceProperties{
					ID: eh.UUID(params["parentid"].(string)),
					Properties: map[string]interface{}{
						"SubSysHealth": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

}
