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

func RegisterCMCAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("manager_cmc_integrated",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Manager.v1_0_2.Manager",
					Context:     "/redfish/v1/$metadata#Manager.Manager",

					Privileges: map[string]interface{}{
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"@odata.etag@meta":         vw.Meta(view.GETProperty("etag"), view.GETModel("etag")),
						"Id@meta":                  vw.Meta(view.PropGET("unique_name")),
						"Name":                     "Manager", //hardcoded in odatalite
						"ManagerType":              "BMC",     //hardcoded in odatalite
						"Description":              "BMC",     //hardcoded in odatalite
						"Model@meta":               vw.Meta(view.PropGET("model")),
						"DateTime@meta":            map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"},"PATCH": map[string]interface{}{"plugin":vw.GetURI(),"property":"time", "controller":"ar_mapper" }},
						"DateTimeLocalOffset@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetimezone"}},
						"FirmwareVersion@meta":     vw.Meta(view.PropGET("firmware_version")),
						"Links": map[string]interface{}{
							//"ManagerForServers@meta": vw.Meta(view.PropGET("bmc_manager_for_servers")), // not in odatalite json?
							//"ManagerInChassis@meta":  vw.Meta(view.PropGET("in_chassis")), //not in odatalite json?
							"ManagerForChassis@meta":             vw.Meta(view.GETProperty("manager_for_chassis"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
							"ManagerForChassis@odata.count@meta": vw.Meta(view.GETProperty("manager_for_chassis"), view.GETFormatter("count"), view.GETModel("default")),
						},

						"SerialConsole": map[string]interface{}{
							"ConnectTypesSupported@meta":             vw.Meta(view.GETProperty("connect_types_supported"), view.GETModel("default")),
							"ConnectTypesSupported@odata.count@meta": vw.Meta(view.GETProperty("connect_types_supported"), view.GETFormatter("count"), view.GETModel("default")),
							"MaxConcurrentSessions":                  0,
							"ServiceEnabled":                         false,
						},

						"CommandShell": map[string]interface{}{
							"ConnectTypesSupported@meta":             vw.Meta(view.GETProperty("connect_types_supported"), view.GETModel("default")),
							"ConnectTypesSupported@odata.count@meta": vw.Meta(view.GETProperty("connect_types_supported"), view.GETFormatter("count"), view.GETModel("default")),
							"MaxConcurrentSessions":                  0,
							"ServiceEnabled":                         false,
						},

						"LogServices": map[string]interface{}{
							"@odata.id": vw.GetURI() + "/LogServices",
						},

						"GraphicalConsole": map[string]interface{}{
							"ConnectTypesSupported@meta":             vw.Meta(view.GETProperty("connect_types_supported"), view.GETModel("default")),
							"ConnectTypesSupported@odata.count@meta": vw.Meta(view.GETProperty("connect_types_supported"), view.GETFormatter("count"), view.GETModel("default")),
							"MaxConcurrentSessions":                  0,
							"ServiceEnabled":                         false,
						},

						"Oem": map[string]interface{}{
							"@odata.type": "#DellManager.v1_0_0.DellManager",
							"OemAttributes": map[string]interface{}{
								"@odata.id": vw.GetURI() + "/Attributes",
							},
							"CertificateService": map[string]interface{}{
								"@odata.id": vw.GetURI() + "/CertificateService",
							},
						},

						"Status": map[string]interface{}{
							"Health@meta":       vw.Meta(view.PropGET("health")),
							"HealthRollup@meta": vw.Meta(view.PropGET("health")),
							"State@meta":        vw.Meta(view.PropGET("health_state")),
						},

						"Actions": map[string]interface{}{
							"#Manager.Reset": map[string]interface{}{
								"target": vw.GetActionURI("manager.reset"),
								"ResetType@Redfish.AllowableValues": []string{
									"GracefulRestart",
								},
							},
							"#Manager.ForceFailover": map[string]interface{}{
								"target": vw.GetActionURI("manager.forcefailover"),
							},
							"Oem": map[string]interface{}{
								"#DellManager.v1_0_0.DellManager.ResetToDefaults": map[string]interface{}{
									"ResetType@Redfish.AllowableValues": []string{
										"ClearToShip",
										"Decommission",
										"ResetFactoryConfig",
										"ResetToEngineeringDefaults",
										"Default",
									},
									"target": vw.GetActionURI("manager.resettodefaults"),
								},
								"OemManager.v1_0_0#OemManager.ExportSystemConfiguration": map[string]interface{}{
									"ExportFormat@Redfish.AllowableValues": []string{
										"XML",
										"JSON",
									},
									"ExportUse@Redfish.AllowableValues": []string{
										"Default",
										"Clone",
										"Replace",
									},
									"IncludeInExport@Redfish.AllowableValues": []string{
										"Default",
										"IncludeReadOnly",
										"IncludePasswordHashValues",
										"IncludeReadOnly,IncludePasswordHashValues",
									},
									"ShareParameters": map[string]interface{}{
										"IgnoreCertificateWarning@Redfish.AllowableValues": []string{
											"Disabled",
											"Enabled",
										},
										"ProxySupport@Redfish.AllowableValues": []string{
											"Disabled",
											"EnabledProxyDefault",
											"Enabled",
										},
										"ProxyType@Redfish.AllowableValues": []string{
											"HTTP",
											"SOCKS4",
										},
										"ShareParameters@Redfish.AllowableValues": []string{
											"IPAddress",
											"ShareName",
											"FileName",
											"UserName",
											"Password",
											"Workgroup",
											"ProxyServer",
											"ProxyUserName",
											"ProxyPassword",
											"ProxyPort",
										},
										"ShareType@Redfish.AllowableValues": []string{
											"NFS",
											"CIFS",
											"HTTP",
											"HTTPS",
										},
										"Target@Redfish.AllowableValues": []string{
											"ALL",
											"IDRAC",
											"BIOS",
											"NIC",
											"RAID",
										},
									},
									"target": vw.GetActionURI("manager.exportsystemconfig"),
								},
								"OemManager.v1_0_0#OemManager.ImportSystemConfiguration": map[string]interface{}{
									"HostPowerState@Redfish.AllowableValues": []string{
										"On",
										"Off",
									},
									"ImportSystemConfiguration@Redfish.AllowableValues": []string{
										"TimeToWait",
										"ImportBuffer",
									},
									"ShareParameters": map[string]interface{}{
										"IgnoreCertificateWarning@Redfish.AllowableValues": []string{
											"Disabled",
											"Enabled",
										},
										"ProxySupport@Redfish.AllowableValues": []string{
											"Disabled",
											"EnabledProxyDefault",
											"Enabled",
										},
										"ProxyType@Redfish.AllowableValues": []string{
											"HTTP",
											"SOCKS4",
										},
										"ShareParameters@Redfish.AllowableValues": []string{
											"IPAddress",
											"ShareName",
											"FileName",
											"UserName",
											"Password",
											"Workgroup",
											"ProxyServer",
											"ProxyUserName",
											"ProxyPassword",
											"ProxyPort",
										},
										"ShareType@Redfish.AllowableValues": []string{
											"NFS",
											"CIFS",
											"HTTP",
											"HTTPS",
										},
										"Target@Redfish.AllowableValues": []string{
											"ALL",
											"IDRAC",
											"BIOS",
											"NIC",
											"RAID",
										},
									},
									"ShutdownType@Redfish.AllowableValues": []string{
										"Graceful",
										"Forced",
										"NoReboot",
									},
									"target": vw.GetActionURI("manager.importsystemconfig"),
								},
								"OemManager.v1_0_0#OemManager.ImportSystemConfigurationPreview": map[string]interface{}{
									"ImportSystemConfigurationPreview@Redfish.AllowableValues": []string{
										"ImportBuffer",
									},
									"ShareParameters": map[string]interface{}{
										"IgnoreCertificateWarning@Redfish.AllowableValues": []string{
											"Disabled",
											"Enabled",
										},
										"ProxySupport@Redfish.AllowableValues": []string{
											"Disabled",
											"EnabledProxyDefault",
											"Enabled",
										},
										"ProxyType@Redfish.AllowableValues": []string{
											"HTTP",
											"SOCKS4",
										},
										"ShareParameters@Redfish.AllowableValues": []string{
											"IPAddress",
											"ShareName",
											"FileName",
											"UserName",
											"Password",
											"Workgroup",
											"ProxyServer",
											"ProxyUserName",
											"ProxyPassword",
											"ProxyPort",
										},
										"ShareType@Redfish.AllowableValues": []string{
											"NFS",
											"CIFS",
											"HTTP",
											"HTTPS",
										},
										"Target@Redfish.AllowableValues": []string{
											"ALL",
										},
									},
									"target": vw.GetActionURI("manager.importsystemconfigpreview"),
								},
							},
						},
						"Redundancy@meta":             vw.Meta(view.GETProperty("redundancy_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"Redundancy@odata.count@meta": vw.Meta(view.GETProperty("redundancy_uris"), view.GETFormatter("count"), view.GETModel("default")),
					}}}, nil
		})

	s.RegisterAggregateFunction("chassis_cmc_integrated",
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
						"@odata.etag@meta":  vw.Meta(view.GETProperty("etag"), view.GETModel("etag")),
						"Id@meta":           vw.Meta(view.PropGET("unique_name")),
						"AssetTag":          nil,
						"SerialNumber@meta": vw.Meta(view.PropGET("serial")),      //uses sys.chas.1 ar value
						"PartNumber@meta":   vw.Meta(view.PropGET("part_number")), //uses sys.chas.1 ar value
						"ChassisType@meta":  vw.Meta(view.PropGET("chassis_type")),
						"Model@meta":        vw.Meta(view.PropGET("model")),
						"Manufacturer@meta": vw.Meta(view.PropGET("manufacturer")),
						"Name@meta":         vw.Meta(view.PropGET("name")),
						"SKU":               nil,
						"Description@meta":  vw.Meta(view.PropGET("description")),
						"Links":             map[string]interface{}{},
						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health")),
							"State@meta":        vw.Meta(view.PropGET("health_state")),
							"Health@meta":       vw.Meta(view.PropGET("health")),
						},
						"IndicatorLED": "Blinking", // static.  MSM does a patch operation and reads from attributes
						"Oem": map[string]interface{}{
							"OemChassis": map[string]interface{}{
								"@odata.id": vw.GetURI() + "/Attributes",
							},
						},
					}}}, nil
		})

	s.RegisterAggregateFunction("manager_cmc_integrated_redundancy",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Redundancy.v1_0_2.Redundancy",
					Context:     "/redfish/v1/$metadata#Redundancy.Redundancy",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Name":                           "ManagerRedundancy",
						"Mode@meta":                      vw.Meta(view.PropGET("redundancy_mode")),
						"MinNumNeeded@meta":              vw.Meta(view.PropGET("redundancy_min")),
						"MaxNumSupported@meta":           vw.Meta(view.PropGET("redundancy_max")),
						"RedundancySet@meta":             vw.Meta(view.GETProperty("redundancy_set"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"RedundancySet@odata.count@meta": vw.Meta(view.GETProperty("redundancy_set"), view.GETFormatter("count"), view.GETModel("default")),
						"Status": map[string]interface{}{
							"Health@meta":       vw.Meta(view.PropGET("health")),
							"HealthRollup@meta": vw.Meta(view.PropGET("health")),
							"State@meta":        vw.Meta(view.PropGET("health_state")),
						},
					},
				}}, nil
		})

}
