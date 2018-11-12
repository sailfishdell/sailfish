package system

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
	s.RegisterAggregateFunction("idrac_system_embedded",
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
						"@odata.context":           "/redfish/v1/$metadata#ComputerSystem.ComputerSystem",
						"@odata.id":                "/redfish/v1/Systems/System.Embedded.1",
						"@odata.type":              "#ComputerSystem.v1_4_0.ComputerSystem",
						"Id":                       "System.Embedded.1",
						"AssetTag":                 "",
						"BiosVersion":              "0.4.1",
						"Description":              "Computer System which represents a machine (physical or virtual) and the local resources such as memory, cpu and other devices that can be accessed from that machine.",
						"HostName":                 "WIN-02GODDHDJTC",
						"HostingRoles":             []string{},
						"HostingRoles@odata.count": 0,
						"IndicatorLED":             "Off",
						"UUID":                     "4c4c4544-0036-3510-8034-b7c04f333231",
						"SystemType":               "Physical",
						"SerialNumber":             "",
						"SKU":                      "7654321",
						"Manufacturer":             "Dell Inc.",
						"Model":                    "PowerEdge R740",
						"Name":                     "System",
						"PartNumber":               "",
						"PowerState":               "On",

						"Status": map[string]interface{}{
							"Health":       "Critical",
							"HealthRollup": "Critical",
							"State":        "Enabled",
						},

						"TrustedModules": []map[string]interface{}{
							{
								"Status": map[string]interface{}{
									"State": "Disabled",
								},
							},
						},

						"MemorySummary": map[string]interface{}{
							"MemoryMirroring": "System",
							"Status": map[string]interface{}{
								"Health":       "OK",
								"HealthRollup": "OK",
								"State":        "Enabled",
							},
							"TotalSystemMemoryGiB": 7.450584,
						},

						"ProcessorSummary": map[string]interface{}{
							"Count": 1,
							"Model": "Intel(R) Xeon(R) Gold 6130 CPU @ 2.10GHz",
							"Status": map[string]interface{}{
								"Health":       "OK",
								"HealthRollup": "OK",
								"State":        "Enabled",
							},
						},

						"Actions": map[string]interface{}{
							"#ComputerSystem.Reset": map[string]interface{}{
								"ResetType@Redfish.AllowableValues": []string{
									"On",
									"ForceOff",
									"GracefulRestart",
									"GracefulShutdown",
									"PushPowerButton",
									"Nmi",
								},
								// TODO: withaction
								//"target": "/redfish/v1/Systems/System.Embedded.1/Actions/ComputerSystem.Reset",
							},
						},
						"Boot": map[string]interface{}{
							"BootSourceOverrideEnabled": "Once",
							"BootSourceOverrideMode":    "UEFI",
							"BootSourceOverrideTarget":  "None",
							"BootSourceOverrideTarget@Redfish.AllowableValues": []string{
								"None",
								"Pxe",
								"Floppy",
								"Cd",
								"Hdd",
								"BiosSetup",
								"Utilities",
								"UefiTarget",
								"SDCard",
								"UefiHttp",
							},
							"UefiTargetBootSourceOverride": "",
						},
						"Links": map[string]interface{}{
							"Chassis@meta":               vw.Meta(view.GETProperty("chassis_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"Chassis@odata.count@meta":   vw.Meta(view.GETProperty("chassis_uris"), view.GETFormatter("count"), view.GETModel("default")),
							"CooledBy@meta":              vw.Meta(view.GETProperty("cooled_by_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"CooledBy@odata.count@meta":  vw.Meta(view.GETProperty("cooled_by_uris"), view.GETFormatter("count"), view.GETModel("default")),
							"ManagedBy@meta":             vw.Meta(view.GETProperty("manager_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"ManagedBy@odata.count@meta": vw.Meta(view.GETProperty("manager_uris"), view.GETFormatter("count"), view.GETModel("default")),
							"PoweredBy@meta":             vw.Meta(view.GETProperty("power_uris"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"PoweredBy@odata.count@meta": vw.Meta(view.GETProperty("power_uris"), view.GETFormatter("count"), view.GETModel("default")),
							"Oem": map[string]interface{}{
								"DELL": map[string]interface{}{
									"@odata.type": "#DellComputerSystem.v1_0_0.DellComputerSystem",
									/*
										// uncomment when this uri is implemented. should be a static instantiate in the system_embedded instantiate
										"BootOrder": map[string]interface{}{
											"@odata.id": "/redfish/v1/Systems/System.Embedded.1/BootSources",
										},
									*/
								},
							},
						},
						/*
							// TODO: uncomment when this is instantiated. should be a static instantiate in the yaml file
							"Bios": map[string]interface{}{
								"@odata.id": "/redfish/v1/Systems/System.Embedded.1/Bios",
							},
						*/
						/*
								// TODO: uncomment when this is instantiated. should be a static instantiate in the yaml file
							"EthernetInterfaces": map[string]interface{}{
								"@odata.id": "/redfish/v1/Systems/System.Embedded.1/EthernetInterfaces",
							},
						*/

						/*
							// TODO: uncomment when this is instantiated. should be a static instantiate in the yaml file
							"Memory": map[string]interface{}{
								"@odata.id": "/redfish/v1/Systems/System.Embedded.1/Memory",
							},
						*/

						/*
							// TODO: uncomment when this is instantiated. should be a static instantiate in the yaml file
							"NetworkInterfaces": map[string]interface{}{
								"@odata.id": "/redfish/v1/Systems/System.Embedded.1/NetworkInterfaces",
							},
						*/
						/*
									// TODO: uncomment when this is instantiated. should be a static instantiate in the yaml file
							"Processors": map[string]interface{}{
								"@odata.id": "/redfish/v1/Systems/System.Embedded.1/Processors",
							},
						*/
						/*
									// TODO: uncomment when this is instantiated. should be a static instantiate in the yaml file
							"SecureBoot": map[string]interface{}{
								"@odata.id": "/redfish/v1/Systems/System.Embedded.1/SecureBoot",
							},
						*/
						/*
									// TODO: uncomment when this is instantiated. should be a static instantiate in the yaml file
							"SimpleStorage": map[string]interface{}{
								"@odata.id": "/redfish/v1/Systems/System.Embedded.1/SimpleStorage/Controllers",
							},
						*/

						/*
									// TODO: uncomment when this is instantiated. should be a static instantiate in the yaml file
							"Storage": map[string]interface{}{
								"@odata.id": "/redfish/v1/Systems/System.Embedded.1/Storage",
							},
						*/
					},
				},
			}, nil
		})
}
