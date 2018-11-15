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
						"GET":   []string{"Unauthenticated"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"@odata.etag@meta":         vw.Meta(view.GETProperty("etag"), view.GETModel("etag")),
						"Id@meta":                  vw.Meta(view.PropGET("unique_name")),
						"Name":                     "Manager", //hardcoded in odatalite
						"ManagerType":              "BMC",     //hardcoded in odatalite
						"Description":              "BMC",     //hardcoded in odatalite
						"Model@meta":               vw.Meta(view.PropGET("model")),
						"DateTime@meta":            map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
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
								// TODO: Remove per JIT-66996
								"DellManager.v1_0_0#DellManager.ResetToDefaults": map[string]interface{}{
									"ResetType@Redfish.AllowableValues": []string{
										"ClearToShip",
										"Decommission",
										"ResetFactoryConfig",
										"ResetToEngineeringDefaults",
									},
									"target": vw.GetActionURI("manager.resettodefaults"),
								},
								"#DellManager.v1_0_0.DellManager.ResetToDefaults": map[string]interface{}{
									"ResetType@Redfish.AllowableValues": []string{
										"ClearToShip",
										"Decommission",
										"ResetFactoryConfig",
										"ResetToEngineeringDefaults",
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

}
