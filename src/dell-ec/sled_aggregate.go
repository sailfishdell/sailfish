package dell_ec

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
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
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Id@meta":           vw.Meta(view.GETProperty("unique_name"), view.GETModel("default")),
						"SKU@meta":          vw.Meta(view.GETProperty("service_tag"), view.GETModel("default")),
						"IndicatorLED@meta": vw.Meta(view.GETModel("default"), view.PropPATCH("indicator_led", "ar_mapper"), view.GETProperty("indicator_led")),
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
							  "OemChassis": map[string]interface{}{
								  "@odata.id": vw.GetURI() + "/Attributes",
							  },
								"InstPowerConsumption": 0,
							},
              "OemChassis": map[string]interface{}{ //TODO: Remove for Redfish Compliance
								"@odata.id": vw.GetURI() + "/Attributes",
							},
						},
						"Actions": map[string]interface{}{
							"Oem": map[string]interface{}{
								"#DellChassis.v1_0_0.PeripheralMapping": map[string]interface{}{
									"MappingType@Redfish.AllowableValues": []string{
										"Accept",
										"Clear",
									},
									"target": vw.GetActionURI("chassis.peripheralmapping"),
								},
								"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
									"target": vw.GetActionURI("chassis.virtualreseat"),
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
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
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
						"PowerState":  "",

						"IndicatorLED@meta": vw.Meta(view.GETModel("default"), view.PropPATCH("indicator_led", "ar_mapper"), view.GETProperty("indicator_led")),
						"SKU@meta":          vw.Meta(view.GETProperty("service_tag"), view.GETModel("default")),

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
									"ForceRestart",
									"GracefulShutdown",
									"GracefulRestart",
								},
								"target": vw.GetActionURI("chassis.reset"),
							},
							"Oem": map[string]interface{}{
								"#DellChassis.v1_0_0.MSMConfigBackup": map[string]interface{}{
									"target": vw.GetUploadURI("msmconfigbackup"),
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
						"Battery": map[string]interface{}{
							"Status": map[string]interface{}{
								"HealthRollup@meta": vw.Meta(view.GETProperty("battery_rollup"), view.GETModel("global_health")),
							},
						},
						"Fan": map[string]interface{}{
							"Status": map[string]interface{}{
								"HealthRollup@meta": vw.Meta(view.GETProperty("fan_rollup"), view.GETModel("global_health")),
							},
						},
						"MM": map[string]interface{}{
							"Status": map[string]interface{}{
								"HealthRollup@meta": vw.Meta(view.GETProperty("mm_rollup"), view.GETModel("global_health")),
							},
						},
						"Miscellaneous": map[string]interface{}{
							"Status": map[string]interface{}{
								"HealthRollup@meta": vw.Meta(view.GETProperty("misc_rollup"), view.GETModel("global_health")),
							},
						},
						"PowerSupply": map[string]interface{}{
							"Status": map[string]interface{}{
								"HealthRollup@meta": vw.Meta(view.GETProperty("psu_rollup"), view.GETModel("global_health")),
							},
						},
						"Temperature": map[string]interface{}{
							"Status": map[string]interface{}{
								"HealthRollup@meta": vw.Meta(view.GETProperty("temperature_rollup"), view.GETModel("global_health")),
							},
						},
					},
				}}, nil
		})

}

func remove(s []string, r string) bool {

	for i, v := range s {
		ml := len(s) - 1
		if v == r {
			tmp := s[ml]
			s[ml] = s[i]
			s[i] = tmp
			s[ml] = ""
			s = s[:ml]
			return true
		}
	}
	return false
}

// Contains tells whether a contains x.
func Contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func inithealth(ctx context.Context, logger log.Logger, ch eh.CommandHandler, d *domain.DomainObjects) {
	sled_iomL := []string{}

	awesome_mapper2.AddFunction("remove_health", func(args ...interface{}) (interface{}, error) {
		removedEvent, ok := args[0].(*dm_event.ComponentRemovedData)
		if !ok {
			logger.Crit("Mapper configuration error: component removed event data not passed", "args[0]", args[0], "TYPE", fmt.Sprintf("%T", args[0]))
			return nil, errors.New("Mapper configuration error: component removed event data not passed")
		}
		aggregateUUID, ok := args[1].(eh.UUID)
		if !ok {
			logger.Crit("Mapper configuration error: aggregate UUID not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("Mapper configuration error: aggregate UUID not passed")
		}
		subsys := removedEvent.Name
		remove(sled_iomL, subsys)

		ch.HandleCommand(ctx,
			&domain.RemoveRedfishResourceProperty{
				ID:       aggregateUUID,
				Property: subsys})

		return nil, nil
	})

	awesome_mapper2.AddFunction("add_health_to_subsystem", func(args ...interface{}) (interface{}, error) {
		// which components can I use to generate subsystem one time..
		healthEvent, ok := args[0].(*dm_event.HealthEventData)
		if !ok {
			logger.Crit("Mapper configuration error: health event data not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("Mapper configuration error: health event data not passed")
		}

		s := strings.Split(healthEvent.FQDD, "#")
		subsys := s[len(s)-1]

		// if iom or sled already in list, exit out, global health automatically updates health
		// TODO update to a function later
		if Contains(sled_iomL, subsys) {
			return nil, nil
		} else {
			sled_iomL = append(sled_iomL, subsys)
		}

		aggregateUUID, ok := args[1].(eh.UUID)
		if !ok {
			logger.Crit("Mapper configuration error: aggregate UUID not passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("Mapper configuration error: aggregate UUID not passed")
		}

		agg, _ := d.AggregateStore.Load(context.Background(), domain.AggregateType, aggregateUUID)

		ch.HandleCommand(ctx,
			&domain.UpdateRedfishResourceProperties{
				ID: aggregateUUID,
				Properties: map[string]interface{}{subsys: map[string]interface{}{
					"Status": map[string]interface{}{
						"HealthRollup@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": agg.(*domain.RedfishResourceAggregate).ResourceURI, "property": healthEvent.FQDD, "model": "global_health"}}}}},
			})

		// to avoid extra memory usage, returning 'nil', but in the future should return subSystemHealthList when we can use it
		return nil, nil
	})

}
