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

	s.RegisterAggregateFunction("idrac_storage_controller",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Storage.v1_4_0.StorageController",
					Context:     "/redfish/v1/$metadata#Storage.Storage",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Assembly": map[string]interface{}{
							"@odata.id": "/redfish/v1/Chassis/System.Embedded.1/Assembly",
						},
						"FirmwareVersion@meta": vw.Meta(view.PropGET("firmware_version")),
						"Identifiers@meta":     vw.Meta(view.GETProperty("identifiers"), view.GETModel("default")),
						"Links":                map[string]interface{}{},
						"Manufacturer@meta":    vw.Meta(view.PropGET("manufacturer")),
						"MemberId@meta":        vw.Meta(view.PropGET("member_id")),
						"Model@meta":           vw.Meta(view.PropGET("model")),
						"Name@meta":            vw.Meta(view.PropGET("name")),
						"SpeedGbps@meta":       vw.Meta(view.PropGET("speed")),
						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health_rollup")),
							"State":             "Enabled",
							"Health@meta":       vw.Meta(view.PropGET("health")),
						},
						"SupportedControllerProtocols@meta": vw.Meta(view.PropGET("supported_controller_protocols")),
						"SupportedDeviceProtocols@meta":     vw.Meta(view.PropGET("supported_device_protocols")),
					}}}, nil
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
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Id@meta":                 vw.Meta(view.PropGET("Id")),
						"Description@meta":        vw.Meta(view.PropGET("description")), //Done
						"Name@meta":               vw.Meta(view.PropGET("name")),        //Done
						"Drives@meta":             vw.Meta(view.GETProperty("drive_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Drives@odata.count@meta": vw.Meta(view.GETProperty("drive_uris"), view.GETFormatter("count"), view.GETModel("default")),

						// volumes is a single static link to a collection
						"Volumes": map[string]interface{}{"@odata.id": vw.GetURI() + "/Volumes"},

						"StorageControllers@meta":             vw.Meta(view.GETProperty("storage_controller_uris_instance"), view.GETFormatter("expand"), view.GETModel("default")),
						"StorageControllers@odata.count@meta": vw.Meta(view.GETProperty("storage_controller_uris_instance"), view.GETFormatter("count"), view.GETModel("default")),

						"Links": map[string]interface{}{
							"Enclosures@meta":             vw.Meta(view.GETProperty("enclosure_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"Enclosures@odata.count@meta": vw.Meta(view.GETProperty("enclosure_uris"), view.GETFormatter("count"), view.GETModel("default")),
						},

						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health_rollup")),
							"State":             "Enabled",
							"Health@meta":       vw.Meta(view.PropGET("health")),
						},

						"Actions": map[string]interface{}{
							"#Storage.SetEncryptionKey": map[string]interface{}{
								"target": vw.GetActionURI("storage.setencryptionkey"),
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

	s.RegisterAggregateFunction("idrac_storage_drive",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Drive.v1_3_0.Drive",
					Context:     "/redfish/v1/$metadata#Drive.Drive",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{"ConfigureManager"},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						// TODO: assembly shouldnt be hard coded
						"Assembly": map[string]interface{}{
							"@odata.id": "/redfish/v1/Chassis/System.Embedded.1/Assembly",
						},
						"Actions": map[string]interface{}{
							"#Drive.SecureErase": map[string]interface{}{
								"target": vw.GetActionURI("drive.secureerase"),
							},
						},
						"BlockSizeBytes@meta":    vw.Meta(view.PropGET("block_size_bytes")),
						"CapableSpeedGbs@meta":   vw.Meta(view.PropGET("capable_speed")),
						"CapacityBytes@meta":     vw.Meta(view.PropGET("capacity")),
						"Description@meta":       vw.Meta(view.PropGET("description")),
						"EncryptionAbility@meta": vw.Meta(view.PropGET("encryption_ability")),
						"EncryptionStatus@meta":  vw.Meta(view.PropGET("encryption_status")),
						"FailurePredicted@meta":  vw.Meta(view.PropGET("failure_predicted")),
						"HotspareType@meta":      vw.Meta(view.PropGET("hotspare_type")),
						"Id@meta":                vw.Meta(view.PropGET("unique_name")),
						"Identifiers@meta":       vw.Meta(view.GETProperty("identifiers"), view.GETModel("default")),
						"Links": map[string]interface{}{
							"Chassis@meta":             vw.Meta(view.GETProperty("enclosure_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"Volumes@meta":             vw.Meta(view.GETProperty("volume_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"Volumes@odata.count@meta": vw.Meta(view.GETProperty("volume_uris"), view.GETFormatter("count"), view.GETModel("default")),
						},
						"Location":                map[string]interface{}{},
						"Manufacturer@meta":       vw.Meta(view.PropGET("manufacturer")),     //Done
						"MediaType@meta":          vw.Meta(view.PropGET("media_type")),       //Done
						"Model@meta":              vw.Meta(view.PropGET("model")),            //Done
						"Name@meta":               vw.Meta(view.PropGET("name")),             //Done
						"NegotiatedSpeedGbs@meta": vw.Meta(view.PropGET("negotiated_speed")), //Done
						"Oem": map[string]interface{}{ //Done
							"Dell": map[string]interface{}{
								"DellPhysicalDisk": map[string]interface{}{
									"@odata.context":              "/redfish/v1/$metadata#DellPhysicalDisk.DellPhysicalDisk",
									"@odata.id":                   "/redfish/v1/Dell/Systems/System.Embedded.1/Storage/Drives/DellPhysicalDisk/$entity",
									"@odata.type":                 "#DellPhysicalDisk.v1_0_0.DellPhysicalDisk",
									"Connector@meta":              vw.Meta(view.PropGET("connector")),
									"DriveFormFactor@meta":        vw.Meta(view.PropGET("drive_formfactor")),
									"FreeSizeInBytes@meta":        vw.Meta(view.PropGET("free_size")),
									"ManufacturingDay@meta":       vw.Meta(view.PropGET("manufacturing_day")),
									"ManufacturingWeek@meta":      vw.Meta(view.PropGET("manufacturing_week")),
									"ManufacturingYear@meta":      vw.Meta(view.PropGET("manufacturing_year")),
									"PPID@meta":                   vw.Meta(view.PropGET("ppid")),
									"PredictiveFailureState@meta": vw.Meta(view.PropGET("predictive_failure_state")),
									"RaidStatus@meta":             vw.Meta(view.PropGET("raid_status")),
									"SASAddress@meta":             vw.Meta(view.PropGET("sas_address")),
									"Slot@meta":                   vw.Meta(view.PropGET("slot")),
									"UsedSizeInBytes@meta":        vw.Meta(view.PropGET("used_size")),
								},
							},
						},
						"Operations":                         []map[string]interface{}{},
						"PartNumber@meta":                    vw.Meta(view.PropGET("part_number")),
						"PredictedMediaLifeLeftPercent@meta": vw.Meta(view.PropGET("predicted_media_life_left_percent")),
						"Protocol@meta":                      vw.Meta(view.PropGET("protocol")),
						"Revision@meta":                      vw.Meta(view.PropGET("revision")),
						"RotationSpeedRPM@meta":              vw.Meta(view.PropGET("rotation_speed")),
						"SerialNumber@meta":                  vw.Meta(view.PropGET("serial_number")),
						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health_rollup")),
							"State":             "Enabled",
							"Health@meta":       vw.Meta(view.PropGET("health")),
						},
					}}}, nil
		})

	s.RegisterAggregateFunction("idrac_storage_enclosure",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{

					ResourceURI: vw.GetURI(),
					Type:        "#Chassis.v1_6_0.Chassis",
					Context:     "/redfish/v1/$metadata#Chassis.Chassis",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{"ConfigureManager"},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{"ConfigureManager"},
					},

					Properties: map[string]interface{}{
						"@Redfish.Settings@meta": vw.Meta(view.GETProperty("settings_uri"), view.GETFormatter("expandone"), view.GETModel("default")),
						"Actions":                map[string]interface{}{},
						"AssetTag@meta":          vw.Meta(view.PropGET("asset_tag")), //Done
						"ChassisType":            "Enclosure",
						"Description@meta":       vw.Meta(view.PropGET("description")),
						"Id@meta":                vw.Meta(view.PropGET("unique_name")),
						"Links": map[string]interface{}{
							"ContainedBy": map[string]interface{}{
								"@odata.id": "/redfish/v1/Chassis/System.Embedded.1",
							},
							//"Contains":vw.Meta(view.PropGET("contains")),
							"Contains":                map[string]interface{}{},
							"Contains@odata.count":    0,
							"Drives@meta":             vw.Meta(view.GETProperty("encl_drv_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"Drives@odata.count@meta": vw.Meta(view.GETProperty("encl_drv_uris"), view.GETFormatter("count"), view.GETModel("default")),
							"ManagedBy": map[string]interface{}{
								"@odata.id": "/redfish/v1/Managers/iDRAC.Embedded.1",
							},
							"ManagedBy@odata.count": 1,
							"PCIeDevices@meta":      map[string]interface{}{
								//Needs addition
							},
							"PCIeDevices@odata.count":  0,
							"Storage@meta":             vw.Meta(view.GETProperty("storage_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"Storage@odata.count@meta": vw.Meta(view.GETProperty("storage_uris"), view.GETFormatter("count"), view.GETModel("default")),
						},
						"Manufacturer@meta": vw.Meta(view.PropGET("manufacturer")), //Done
						"Model@meta":        vw.Meta(view.PropGET("model")),        //Done
						"Name@meta":         vw.Meta(view.PropGET("name")),         //Done
						"Oem": map[string]interface{}{ //Done
							"Dell": map[string]interface{}{
								//"DellEnclosure@meta": vw.Meta(view.GETProperty("enclosure_uris"), view.GETFormatter("expandone"), view.GETModel("default")),
								"DellEnclosure": map[string]interface{}{
									"@odata.context":  "/redfish/v1/$metadata#DellEnclosure.DellEnclosure",
									"@odata.id":       "/redfish/v1/Dell/Chassis/System.Embedded.1/DellEnclosure/$entity",
									"@odata.type":     "#DellEnclosure.v1_0_0.DellEnclosure",
									"Connector@meta":  vw.Meta(view.PropGET("connector")),
									"Links@meta":      vw.Meta(view.PropGET("")),
									"ServiceTag@meta": vw.Meta(view.PropGET("service_tag")),
									"SlotCount@meta":  vw.Meta(view.PropGET("slot_count")),
									"Version@meta":    vw.Meta(view.PropGET("version")),
									"WiredOrder@meta": vw.Meta(view.PropGET("wired_order")),
								},
							},
						},
						"PCIeDevices":                  map[string]interface{}{},
						"PCIeDevices@odata.count@meta": vw.Meta(view.PropGET("pcie_devices_count")),
						"PartNumber@meta":              vw.Meta(view.PropGET("part_number")),
						"PowerState@meta":              vw.Meta(view.PropGET("power_state")),
						"SKU@meta":                     vw.Meta(view.PropGET("sku")),
						"SerialNumber@meta":            vw.Meta(view.PropGET("serial_number")),
						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health_rollup")),
							"State":             "Enabled",
							"Health@meta":       vw.Meta(view.PropGET("health")),
						},
					},
				},
			}, nil
		})

	s.RegisterAggregateFunction("idrac_storage_volume_collection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#VolumeCollection.VolumeCollection",
					Context:     params["rooturi"].(string) + "/$metadata#VolumeCollection.VolumeCollection",
					Plugin:      "GenericActionHandler",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{"ConfigureManager"},
						"DELETE": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Name":                     "Volume Collection",
						"Description":              "Collection of Volumes",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	s.RegisterAggregateFunction("idrac_storage_volume",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Volume.v1_0_3.Volume",
					Context:     "/redfish/v1/$metadata#Volume.Volume",
					Plugin:      "GenericActionHandler",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{"ConfigureManager"},
						"DELETE": []string{"ConfigureManager"},
					},

					//Need to add actions
					Properties: map[string]interface{}{
						"@Redfish.Settings@meta": vw.Meta(view.GETProperty("settings_uri"), view.GETFormatter("expandone"), view.GETModel("default")),
						"Actions": map[string]interface{}{
							"#Volume.Delete": map[string]interface{}{

								"target": vw.GetActionURI("volume.delete"),
							},
							"#Volume.CheckConsistency": map[string]interface{}{

								"target": vw.GetActionURI("volume.checkconsistency"),
							},
							"#Volume.Initialize": map[string]interface{}{
								"InitializeType@Redfish.AllowableValues": []string{
									"Fast",
									"Slow",
								},
								"target": vw.GetActionURI("volume.initialize"),
							},
						},
						"BlockSizeBytes@meta":  vw.Meta(view.PropGET("block_size")),  //Done
						"CapacityBytes@meta":   vw.Meta(view.PropGET("capacity")),    //Done
						"Description@meta":     vw.Meta(view.PropGET("description")), //Done
						"Encrypted@meta":       vw.Meta(view.PropGET("encrypted")),   //DONE
						"EncryptionTypes@meta": vw.Meta(view.PropGET("encryptiontypes")),
						"Id@meta":              vw.Meta(view.PropGET("unique_name")),
						"Identifiers@meta":     vw.Meta(view.GETProperty("identifiers"), view.GETModel("default")),
						"Links": map[string]interface{}{
							"Drives@meta":             vw.Meta(view.GETProperty("drive_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"Drives@odata.count@meta": vw.Meta(view.GETProperty("drive_uris"), view.GETFormatter("count"), view.GETModel("default")),
						},
						"Name@meta": vw.Meta(view.PropGET("name")), //Done
						"Oem": map[string]interface{}{ //Done
							"Dell": map[string]interface{}{
								"DellVirtualDisk": map[string]interface{}{
									"@odata.context": "/redfish/v1/$metadata#DellVirtualDisk.DellVirtualDisk",
									"@odata.id":      "/redfish/v1/Dell/Systems/System.Embedded.1/Storage/Volumes/DellVirtualDisk/", //+ 'vw.Meta(view.PropGET("unique_name"))',
									"@odata.type":    "#DellVirtualDisk.v1_0_0.DellVirtualDisk",

									"BusProtocol@meta":         vw.Meta(view.PropGET("bus_protocol")),
									"Cachecade@meta":           vw.Meta(view.PropGET("cache_cade")),
									"DiskCachePolicy@meta":     vw.Meta(view.PropGET("disk_cache_policy")),
									"LockStatus@meta":          vw.Meta(view.PropGET("lock_status")),
									"MediaType@meta":           vw.Meta(view.PropGET("media_type")),
									"ReadCachePolicy@meta":     vw.Meta(view.PropGET("read_cache_policy")),
									"SpanDepth@meta":           vw.Meta(view.PropGET("span_depth")),
									"SpanLength@meta":          vw.Meta(view.PropGET("span_length")),
									"VirtualDiskTargetID@meta": vw.Meta(view.PropGET("virtual_disk_target")),
									"WriteCachePolicy@meta":    vw.Meta(view.PropGET("write_cache_policy")),
								},
							},
						},
						"Operations": []map[string]interface{}{
							//Need to add Operations
						},
						"OptimumIOSizeBytes@meta": vw.Meta(view.PropGET("optimum_io_size_bytes")),
						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health_rollup")),
							"State":             "Enabled",
							"Health@meta":       vw.Meta(view.PropGET("health")),
						},
						"VolumeType@meta": vw.Meta(view.PropGET("volume_type")),
					}}}, nil
		})

	s.RegisterAggregateFunction("idrac_storage_enclosure_settings",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Settings.v1_1_0.Settings",
					Context:     params["rooturi"].(string) + "/$metadata#Settings.Settings",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"SettingsObject": []string{vw.GetURI()},
						"SupportedApplyTimes": []string{
							"Immediate",
							"OnReset",
							"AtMaintenanceWindowStart",
							"InMaintenanceWindowOnReset",
						},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("idrac_storage_volume_settings",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Settings.v1_1_0.Settings",
					Context:     params["rooturi"].(string) + "/$metadata#Settings.Settings",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"SettingsObject": []string{vw.GetURI()},
						"SupportedApplyTimes": []string{
							"Immediate",
							"OnReset",
							"AtMaintenanceWindowStart",
							"InMaintenanceWindowOnReset",
						},
					}},
			}, nil
		})
}
