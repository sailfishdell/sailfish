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

func RegisterIOMAggregate(s *testaggregate.Service) {

	s.RegisterAggregateFunction("iom",
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
						"Id@meta":           vw.Meta(view.PropGET("unique_name")),
						"AssetTag@meta":     vw.Meta(view.PropGET("asset_tag")),
						"Description@meta":  vw.Meta(view.PropGET("description")),
						"PowerState@meta":   vw.Meta(view.PropGET("power_state")),
						"SerialNumber@meta": vw.Meta(view.PropGET("serial")),
						"PartNumber@meta":   vw.Meta(view.PropGET("part_number")),
						"ChassisType@meta":  vw.Meta(view.PropGET("chassis_type")),
						"Model@meta":        vw.Meta(view.PropGET("model")),
						"Name@meta":         vw.Meta(view.PropGET("name")),
						"Manufacturer@meta": vw.Meta(view.PropGET("manufacturer")),

						"Links": map[string]interface{}{
							"ManagedBy@meta":             vw.Meta(view.GETProperty("managed_by"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"ManagedBy@odata.count@meta": vw.Meta(view.GETProperty("managed_by"), view.GETFormatter("count"), view.GETModel("default")),
						},

						"SKU@meta":          vw.Meta(view.PropGET("service_tag")),
						"IndicatorLED@meta": vw.Meta(view.GETProperty("indicator_led"), view.GETModel("default"), view.PropPATCH("indicator_led", "ar_mapper")),
						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health")),
							"State":             "Enabled", //hard coded
							"Health@meta":       vw.Meta(view.PropGET("health")),
						},
						"Oem": map[string]interface{}{
							"Dell": map[string]interface{}{
								"ServiceTag@meta":           vw.Meta(view.PropGET("service_tag")),
								"InstPowerConsumption@meta": vw.Meta(view.PropGET("Instantaneous_Power")),
								"OemChassis": map[string]interface{}{
									"@odata.id": vw.GetURI() + "/Attributes",
								},
								"OemIOMConfiguration": map[string]interface{}{
									"@odata.id": vw.GetURI() + "/IOMConfiguration",
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
								"target": vw.GetActionURI("iom.chassis.reset"),
							},
							"Oem": map[string]interface{}{
								"#DellChassis.v1_0_0.ResetPeakPowerConsumption": map[string]interface{}{
									"target": vw.GetActionURI("iom.resetpeakpowerconsumption"),
								},
								"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
									"target": vw.GetActionURI("iom.virtualreseat"),
								},
								// TODO: Remove per JIT-66996
								"DellChassis.v1_0_0#DellChassis.ResetPeakPowerConsumption": map[string]interface{}{
									"target": vw.GetActionURI("iom.resetpeakpowerconsumption"),
								},
								// TODO: Remove per JIT-66996
								"DellChassis.v1_0_0#DellChassis.VirtualReseat": map[string]interface{}{
									"target": vw.GetActionURI("iom.virtualreseat"),
								},
							},
						},
					}}}, nil
		})

	s.RegisterAggregateFunction("iom_config",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#DellIomConfiguration.v1_0_0.DellIomConfiguration",
					Context:     "/redfish/v1/$metadata#ChassisCollection.ChassisCollection/Members/$entity",
					Plugin:      "GenericActionHandler",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Id@meta":                       vw.Meta(view.PropGET("unique_name")),
						"internal_mgmt_supported@meta":  vw.Meta(view.PropGET("managed")),
						"IOMConfig_objects@meta":        vw.Meta(view.PropGET("config")),
						"Capabilities@meta":             vw.Meta(view.PropGET("capabilities")),
						"Capabilities@odata.count@meta": vw.Meta(view.PropGET("capCount")),
					}}}, nil
		})

}
