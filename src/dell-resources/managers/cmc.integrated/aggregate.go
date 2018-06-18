package cmc_integrated

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"

	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/health"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	ah "github.com/superchalupa/go-redfish/src/actionhandler"
)

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler, eb eh.EventBus, ew waiter) *view.View {

	properties := map[string]interface{}{
		"Id@meta":   v.Meta(view.PropGET("unique_name")),
		"Name@meta": v.Meta(view.PropGET("name")),
		// TODO: is this in AR somewhere?
		"ManagerType":              "BMC",
		"Description@meta":         v.Meta(view.PropGET("description")),
		"Model@meta":               v.Meta(view.PropGET("model")),
		"DateTime@meta":            map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
		"DateTimeLocalOffset@meta": v.Meta(view.PropGET("timezone")),
		"FirmwareVersion@meta":     v.Meta(view.PropGET("firmware_version")),
		"Links": map[string]interface{}{
			"ManagerForServers@meta": v.Meta(view.PropGET("bmc_manager_for_servers")),
			// TODO: Need standard method to count arrays
			// "ManagerForChassis@odata.count": 1,
			"ManagerForChassis@meta": v.Meta(view.PropGET("bmc_manager_for_chassis")),
			"ManagerInChassis@meta":  v.Meta(view.PropGET("in_chassis")),
		},

		"SerialConsole": map[string]interface{}{
			"ConnectTypesSupported@odata.count": "TEST_VALUE",
			"MaxConcurrentSessions":             "TEST_VALUE",
			"ConnectTypesSupported":             []interface{}{},
			"ServiceEnabled":                    false,
		},

		"CommandShell": map[string]interface{}{
			"ConnectTypesSupported@odata.count": "TEST_VALUE",
			"MaxConcurrentSessions":             "TEST_VALUE",
			"ConnectTypesSupported":             []interface{}{},
			"ServiceEnabled":                    false,
		},

		"LogServices": map[string]interface{}{
			"@odata.id": v.GetURI() + "/LogServices",
		},

		"GraphicalConsole": map[string]interface{}{
			"ConnectTypesSupported@odata.count": "TEST_VALUE",
			"MaxConcurrentSessions":             "TEST_VALUE",
			"ConnectTypesSupported":             []interface{}{},
			"ServiceEnabled":                    false,
		},

		"Oem": map[string]interface{}{
			"@odata.type": "#DellManager.v1_0_0.DellManager",
			"OemAttributes": map[string]interface{}{
				"@odata.id": v.GetURI() + "/Attributes",
			},
			"CertificateService": map[string]interface{}{
				"@odata.id": v.GetURI() + "/CertificateService",
			},
		},

		"Actions": map[string]interface{}{
			"#Manager.Reset": map[string]interface{}{
				"target": v.GetURI() + "/Actions/Manager.Reset",
				"ResetType@Redfish.AllowableValues": []string{
					"GracefulRestart",
				},
			},
			"Oem": map[string]interface{}{
				"DellManager.v1_0_0#DellManager.ResetToDefaults": map[string]interface{}{
					"ResetType@Redfish.AllowableValues": []string{
						"ClearToShip",
						"Decommission",
						"ResetFactoryConfig",
						"ResetToEngineeringDefaults",
					},
					"target": v.GetURI() + "/Actions/Oem/DellManager.ResetToDefaults",
				},
				"#Manager.ForceFailover": map[string]interface{}{
					"target": v.GetURI() + "/Actions/Manager.ForceFailover",
				},
				"#DellManager.v1_0_0.DellManager.ResetToDefaults": map[string]interface{}{
					"ResetType@Redfish.AllowableValues": []string{
						"ClearToShip",
						"Decommission",
						"ResetFactoryConfig",
						"ResetToEngineeringDefaults",
					},
					"target": v.GetURI() + "/Actions/Oem/DellManager.ResetToDefaults",
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
					"OemManager.v1_0_0#OemManager.ExportSystemConfiguration": []string{
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
					"target": v.GetURI() + "/Actions/Oem/EID_674_Manager.ExportSystemConfiguration",
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
					"target": v.GetURI() + "/Actions/Oem/EID_674_Manager.ImportSystemConfiguration",
				},
				"OemManager.v1_0_0#OemManager.ImportSystemConfigurationPreview": map[string]interface{}{
					"ImportSystemConfigurationPreview@Redfish.AllowableVaues": []string{
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
					"target": v.GetURI() + "/Actions/Oem/EID_674_Manager.ImportSystemConfigurationPreview",
				},
			},
		},
	}

	// TODO: move this out
	redundancy := map[string]interface{}{
		"@odata.type": "#Redundancy.v1_0_2.Redundancy",
		"RedundancySet": []interface{}{
			map[string]interface{}{
				"@odata.id": "/redfish/v1/Managers/CMC.Integrated.1",
			},
			map[string]interface{}{
				"@odata.id": "/redfish/v1/Managers/CMC.Integrated.2",
			},
		},
		"Name": "ManagerRedundancy",
		"RedundancySet@odata.count": 2,
		"@odata.id":                 "/redfish/v1/Managers/CMC.Integrated.1#Redundancy",
		"@odata.context":            "/redfish/v1/$metadata#Redundancy.Redundancy",
		"Mode@meta":                 v.Meta(view.PropGET("redundancy_mode")),
		"MinNumNeeded@meta":         v.Meta(view.PropGET("redundancy_min")),
		"MaxNumSupported@meta":      v.Meta(view.PropGET("redundancy_max")),
	}
	health.GetHealthFragment(v, "health", properties)
	health.GetHealthFragment(v, "redundancy_health", redundancy)
	properties["Redundancy"] = []interface{}{redundancy}
	properties["Redundancy@odata.count"] = len(properties["Redundancy"].([]interface{}))

	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#Manager.v1_0_2.Manager",
			Context:     "/redfish/v1/$metadata#Manager.Manager",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: properties,
		})

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		v.GetURI()+"/Actions/Manager.Reset",
		"manager.reset",
		v.GetModel("default"))

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		v.GetURI()+"/Actions/Oem/DellManager.ResetToDefaults",
		"manager.resettodefaults",
		v.GetModel("default"))

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		v.GetURI()+"/Actions/Manager.ForceFailover",
		"manager.forcefailover",
		v.GetModel("default"))

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		v.GetURI()+"/Actions/Oem/EID_674_Manager.ExportSystemConfiguration",
		"manager.exportsystemconfiguration",
		v.GetModel("default"))

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		v.GetURI()+"/Actions/Oem/EID_674_Manager.ImportSystemConfiguration",
		"manager.importsystemconfiguration",
		v.GetModel("default"))

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		v.GetURI()+"/Actions/Oem/EID_674_Manager.ImportSystemConfigurationPreview",
		"manager.importsystemconfigurationpreview",
		v.GetModel("default"))

	return v
}
