package storage

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

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("idrac_storage_collection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#StorageCollection.StorageCollection",
					Context:     params["rooturi"].(string) + "/$metadata#StorageCollection.StorageCollection",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Name":                     "Storage  Collection",
						"Description":              "Collection of Storage Devices",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	s.RegisterAggregateFunction("idrac_storage_instance",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Storage.v1_3_0.Storage",
					Context:     "/redfish/v1/$metadata#Storage.Storage",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{"ConfigureManager"},
						"PUT":    []string{},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Id@meta":                  vw.Meta(view.PropGET("unique_name")),
						"Description@meta":         vw.Meta(view.PropGET("description")), //Done
						"Name@meta":                vw.Meta(view.PropGET("name")),        //Done
						"Drives@meta":              vw.Meta(view.GETProperty("drive_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Drives@odata.count@meta":  vw.Meta(view.GETProperty("drive_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"Volumes@meta":             vw.Meta(view.GETProperty("volume_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Volumes@odata.count@meta": vw.Meta(view.GETProperty("volume_uris"), view.GETFormatter("count"), view.GETModel("default")),
						// this should expand:
						"StorageControllers@meta":             vw.Meta(view.GETProperty("storage_controller_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"StorageControllers@odata.count@meta": vw.Meta(view.GETProperty("storage_controller_uris"), view.GETFormatter("count"), view.GETModel("default")),

						"Links": map[string]interface{}{
							"Enclosures@meta":             vw.Meta(view.GETProperty("enclosure_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"Enclosures@odata.count@meta": vw.Meta(view.GETProperty("enclosure_uris"), view.GETFormatter("count"), view.GETModel("default")),
						},

						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("obj_status")),
							"State@meta":        vw.Meta(view.PropGET("state")),
							"Health@meta":       vw.Meta(view.PropGET("obj_status")),
						},

						"Actions": map[string]interface{}{
							"#Storage.SetEncryptionKey": map[string]interface{}{
								"target": "/redfish/v1/Systems/System.Embedded.1/Storage/AHCI.Embedded.1-1/Actions/Storage.SetEncryptionKey",
							},
						},

						"Oem": map[string]interface{}{ //Done
							"Dell": map[string]interface{}{
								"DellController": map[string]interface{}{
									// MEB: WTF is this? it's not right. we should not have an @odata.id/type/context here, ever.
									"@odata.context":                 "/redfish/v1/$metadata#DellController.DellController",
									"@odata.id":                      "/redfish/v1/Dell/Systems/System.Emdedded.1/Storage/DellController/$entity",
									"@odata.type":                    "#DellController.v1_0_0.DellController",
									"CacheSizeInMB@meta":             vw.Meta(view.PropGET("cache_size")),
									"CachecadeCapability@meta":       vw.Meta(view.PropGET("cache_capability")),
									"ControllerFirmwareVersion@meta": vw.Meta(view.PropGET("controller_firmware_version")),
									"DeviceCardSlotType@meta":        vw.Meta(view.PropGET("device_card_slot_type")),
									"DriverVersion@meta":             vw.Meta(view.PropGET("driver_version")),
									"EncryptionCapability@meta":      vw.Meta(view.PropGET("encryption_capability")),
									"EncryptionMode@meta":            vw.Meta(view.PropGET("encryption_mode")),
									"PCISlot@meta":                   vw.Meta(view.PropGET("pci_slot")),
									"PatrolReadState@meta":           vw.Meta(view.PropGET("patrol_read_state")),
									"RollupStatus@meta":              vw.Meta(view.PropGET("rollup_status")),
									"SecurityStatus@meta":            vw.Meta(view.PropGET("security_status")),
									"SlicedVDCapability@meta":        vw.Meta(view.PropGET("sliced_vd_capabiltiy")),
								},
							},
						},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("idrac_storage_enclosure",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{

					ResourceURI: vw.GetURI(),
					Type:        "#Chassis.v1_6_0.Chassis",
					Context:     "/redfish/v1/$metadata#Chassis.Chassis",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},

					Properties: map[string]interface{}{
						"@Redfish.Settings": map[string]interface{}{ //Done
							"@odata.context": "/redfish/v1/$metadata#Settings.Settings",
							"@odata.id":      "/redfish/v1/Chassis/$Entity/Settings",
							"@odata.type":    "#Settings.v1_1_0.Settings",
							"SupportedApplyTimes": []string{
								"Immediate",
								"OnReset",
								"AtMaintenanceWindowStart",
								"InMaintenanceWindowOnReset",
							},
						},
						"Actions":          map[string]interface{}{},
						"AssetTag@meta":    vw.Meta(view.PropGET("asset_tag")), //Done
						"ChassisType@meta": vw.Meta(view.PropGET("chassis_type")),
						"Description@meta": vw.Meta(view.PropGET("description")),
						"Id@meta":          vw.Meta(view.PropGET("unique_name")),
						"Links":            map[string]interface{}{
							//Need to add links detail.
						},
						"Links@meta":             vw.Meta(view.GETProperty("links_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"Links@odata.count@meta": vw.Meta(view.GETProperty("links_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"Manufacturer@meta":      vw.Meta(view.PropGET("manufacturer")), //Done
						"Model@meta":             vw.Meta(view.PropGET("model")),        //Done
						"Name@meta":              vw.Meta(view.PropGET("name")),         //Done
						"Oem": map[string]interface{}{ //Done
							"Dell": map[string]interface{}{
								"DellEnclosure@meta": vw.Meta(view.GETProperty("enclosure_uris"), view.GETFormatter("expandone"), view.GETModel("default")),
							},
						},
						"PCIeDevices":                  map[string]interface{}{},
						"PCIeDevices@odata.count@meta": vw.Meta(view.PropGET("pcie_devices_count")),
						"PartNumber@meta":              vw.Meta(view.PropGET("part_number")),
						"PowerState@meta":              vw.Meta(view.PropGET("power_state")),
						"SKU@meta":                     vw.Meta(view.PropGET("sku")),
						"SerialNumber@meta":            vw.Meta(view.PropGET("serial_number")),
						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health")),
							"State@meta":        vw.Meta(view.PropGET("health_state")),
							"Health@meta":       vw.Meta(view.PropGET("health")),
						},
					},
				},
			}, nil
		})

}
