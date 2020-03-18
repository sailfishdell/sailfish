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
						"GET":   []string{"Login"},
						"POST":  []string{"ConfigureManager"}, // cannot create sub objects
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Id@meta":           vw.Meta(view.PropGET("unique_name")),
						"AssetTag@meta":     vw.Meta(view.PropGET("asset_tag"), view.PropPATCH("asset_tag", "ar_mapper")),
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

						"SKU@meta":          vw.Meta(view.PropGET("service_tag"), view.PropPATCH("service_tag", "ar_mapper")),
						"IndicatorLED@meta": vw.Meta(view.GETProperty("indicator_led"), view.GETModel("default"), view.PropPATCH("indicator_led", "ar_mapper")),
						"Status": map[string]interface{}{
							"HealthRollup": nil,
							"State":        "Enabled", //hard coded
							"Health":       nil,
						},
						"Oem": map[string]interface{}{
							"Dell": map[string]interface{}{
								"ServiceTag@meta":      vw.Meta(view.PropGET("service_tag"), view.PropPATCH("service_tag", "ar_mapper")),
								"InstPowerConsumption": 0,
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
									"ForceOff",
									"ForceRestart",
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
								"#DellChassis.v1_0_0.GetSSOInfo": map[string]interface{}{
									"target": vw.GetActionURI("iom.getssoinfo"),
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
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					DefaultFilter: "$select=!IOMConfig_objects/sso_info", //remove when ready
					Properties: map[string]interface{}{
						"Id":                       0,
						"internal_mgmt_supported":  "",
						"IOMConfig_objects":        []interface{}{},
						"Capabilities":             []interface{}{}, //remove when ready
						"Capabilities@odata.count": 0,
					}}}, nil
		})

}
