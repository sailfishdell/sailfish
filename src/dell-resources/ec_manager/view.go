package ec_manager

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"
	"time"

	"github.com/superchalupa/go-redfish/src/log"
	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	ah "github.com/superchalupa/go-redfish/src/actionhandler"
)

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          s.GetUUID(),
			Collection:  false,
			ResourceURI: s.GetOdataID(),
			Type:        "#Manager.v1_0_2.Manager",
			Context:     "/redfish/v1/$metadata#Manager.Manager",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":                       s.GetProperty("unique_name"),
				"Name@meta":                s.Meta(plugins.PropGET("name")),
				"ManagerType":              "BMC",
				"Description@meta":         s.Meta(plugins.PropGET("description")),
				"Model@meta":               s.Meta(plugins.PropGET("model")),
				"DateTime@meta":            map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
				"DateTimeLocalOffset@meta": s.Meta(plugins.PropGET("timezone"), plugins.PropPATCH("timezone")),
				"FirmwareVersion@meta":     s.Meta(plugins.PropGET("version")),
				"Links": map[string]interface{}{
					"ManagerForServers@meta": s.Meta(plugins.PropGET("bmc_manager_for_servers")),
					// TODO: Need standard method to count arrays
					// "ManagerForChassis@odata.count": 1,
					"ManagerForChassis@meta": s.Meta(plugins.PropGET("bmc_manager_for_chassis")),
					"ManagerInChassis@meta":  s.Meta(plugins.PropGET("in_chassis")),
				},

				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State":        "StandbySpare",
					"Health":       "OK",
				},

				"Redundancy@odata.count": 1,
				"Redundancy": []map[string]interface{}{
					{
						"@odata.type": "#Redundancy.v1_0_2.Redundancy",
						"Status": map[string]interface{}{
							"HealthRollup": "OK",
							"State":        "StandbySpare",
							"Health":       "OK",
						},
						"RedundancySet": []map[string]interface{}{
							map[string]interface{}{
								"@odata.id": "/redfish/v1/Managers/CMC.Integrated.1",
							},
							map[string]interface{}{
								"@odata.id": "/redfish/v1/Managers/CMC.Integrated.2",
							},
						},
						"Name": "ManagerRedundancy",
						"RedundancySet@odata.count": 2,
						"@odata.id":                 "/redfish/v1/Managers/CMC.Integrated.1/Redundancy",
						"@odata.context":            "/redfish/v1/$metadata#Redundancy.Redundancy",
						"Mode":                      "Failover",
						"MinNumNeeded":              2,
						"MaxNumSupported":           2,
					},
				},
				"SerialConsole": map[string]interface{}{
					"ConnectTypesSupported@odata.count": 0,
					"MaxConcurrentSessions":             0,
					"ConnectTypesSupported":             []map[string]interface{}{},
					"ServiceEnabled":                    false,
				},

				"CommandShell": map[string]interface{}{
					"ConnectTypesSupported@odata.count": 0,
					"MaxConcurrentSessions":             0,
					"ConnectTypesSupported":             []map[string]interface{}{},
					"ServiceEnabled":                    false,
				},

				"Actions": map[string]interface{}{
					"#Manager.Reset": map[string]interface{}{
						"target": s.GetOdataID() + "/Actions/Manager.Reset",
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
							"target": "/redfish/v1/Managers/CMC.Integrated.1/Actions/Oem/DellManager.ResetToDefaults",
						},
						"#Manager.ForceFailover": map[string]interface{}{
							"target": "/redfish/v1/Managers/CMC.Integrated.1/Actions/Manager.ForceFailover",
						},
						"#DellManager.v1_0_0.DellManager.ResetToDefaults": map[string]interface{}{
							"ResetType@Redfish.AllowableValues": []string{
								"ClearToShip",
								"Decommission",
								"ResetFactoryConfig",
								"ResetToEngineeringDefaults",
							},
							"target": "/redfish/v1/Managers/CMC.Integrated.1/Actions/Oem/DellManager.ResetToDefaults",
						},
					},
				},

				"LogServices": map[string]interface{}{
					"@odata.id": "/redfish/v1/Managers/CMC.Integrated.1/LogServices",
				},

				"GraphicalConsole": map[string]interface{}{
					"ConnectTypesSupported@odata.count": 0,
					"MaxConcurrentSessions":             0,
					"ConnectTypesSupported":             []map[string]interface{}{},
					"ServiceEnabled":                    false,
				},

				"Oem": map[string]interface{}{
					"@odata.type": "#DellManager.v1_0_0.DellManager",
					"OemAttributes": map[string]interface{}{
						"@odata.id": "/redfish/v1/Managers/CMC.Integrated.1/Attributes",
					},
					"CertificateService": map[string]interface{}{
						"@odata.id": "/redfish/v1/Managers/CMC.Integrated.1/CertificateService",
					},
				},

				/*
					******************************************************
					 DISABLED FOR NOW
					******************************************************
									   "Actions": {
									        "Oem": {
									                       "OemManager.v1_0_0#OemManager.ImportSystemConfigurationPreview": {
									                  "ImportSystemConfigurationPreview@Redfish.AllowableValues": [
									                       "ImportBuffer"
									                  ],
									                  "target": "/redfish/v1/Managers/CMC.Integrated.1/Actions/Oem/EID_674_Manager.ImportSystemConfigurationPreview",
									                  "ShareParameters": {
									                       "ProxySupport XXXXXX
									                            "Disabled",
									                            "EnabledProxyDefault XXXXXX
									                            "Enabled"
									                       ],
									                       "IgnoreCertificateWarning@Redfish.AllowableValues": [
									                            "Disabled",
									                            "Enabled"
									                       ],
									                       "ProxyType XXXXXX
									                            "HTTP",
									                            "SOCKS4"
									                       ],
									                       "ShareType@Redfish.AllowableValues": [
									                            "NFS",
									                            "CIFS",
									                            "HTTP",
									                            "HTTPS"
									                       ],
									                       "Target@Redfish.AllowableValues": [
									                            "ALL"
									                       ],
									                       "ShareParameters@Redfish.AllowableValues": [
									                            "IPAddress XXXXXX
									                            "ShareName",
									                            "FileName",
									                            "UserName",
									                            "Password XXXXXX
									                            "Workgroup XXXXXX
									                            "ProxyServer XXXXXX
									                            "ProxyUserName XXXXXX
									                            "ProxyPassword XXXXXX
									                            "ProxyPort XXXXXX
									                       ]
									                  }
									             },
									             "OemManager.v1_0_0#OemManager.ImportSystemConfiguration": {
									                  "HostPowerState@Redfish.AllowableValues": [
									                       "On",
									                       "Off"
									                  ],
									                  "target": "/redfish/v1/Managers/CMC.Integrated.1/Actions/Oem/EID_674_Manager.ImportSystemConfiguration",
									                  "ShutdownType@Redfish.AllowableValues": [
									                       "Graceful",
									                       "Forced",
									                       "NoReboot"
									                  ],
									                  "ShareParameters": {
									                       "ProxySupport XXXXXX
									                            "Disabled",
									                            "EnabledProxyDefault XXXXXX
									                            "Enabled"
									                       ],
									                       "IgnoreCertificateWarning@Redfish.AllowableValues": [
									                            "Disabled",
									                            "Enabled"
									                       ],
									                       "ProxyType XXXXXX
									                            "HTTP",
									                            "SOCKS4"
									                       ],
									                       "ShareType@Redfish.AllowableValues": [
									                            "NFS",
									                            "CIFS",
									                            "HTTP",
									                            "HTTPS"
									                       ],
									                       "Target@Redfish.AllowableValues": [
									                            "ALL",
									                            "IDRAC",
									                            "BIOS",
									                            "NIC",
									                            "RAID"
									                       ],
									                       "ShareParameters@Redfish.AllowableValues": [
									                            "IPAddress XXXXXX
									                            "ShareName",
									                            "FileName",
									                            "UserName",
									                            "Password XXXXXX
									                            "Workgroup XXXXXX
									                            "ProxyServer XXXXXX
									                            "ProxyUserName XXXXXX
									                            "ProxyPassword XXXXXX
									                            "ProxyPort XXXXXX
									                       ]
									                  },
									                  "ImportSystemConfiguration@Redfish.AllowableValues": [
									                       "TimeToWait",
									                       "ImportBuffer"
									                  ]
									             },
									             "OemManager.v1_0_0#OemManager.ExportSystemConfiguration": {
									                  "IncludeInExport@Redfish.AllowableValues": [
									                       "Default",
									                       "IncludeReadOnly",
									                       "IncludePasswordHashValues XXXXXX
									                       "IncludeReadOnly,IncludePasswordHashValues XXXXXX
									                  ],
									                  "target": "/redfish/v1/Managers/CMC.Integrated.1/Actions/Oem/EID_674_Manager.ExportSystemConfiguration",
									                  "ShareParameters": {
									                       "ProxySupport XXXXXX
									                            "Disabled",
									                            "EnabledProxyDefault XXXXXX
									                            "Enabled"
									                       ],
									                       "IgnoreCertificateWarning@Redfish.AllowableValues": [
									                            "Disabled",
									                            "Enabled"
									                       ],
									                       "ProxyType XXXXXX
									                            "HTTP",
									                            "SOCKS4"
									                       ],
									                       "ShareType@Redfish.AllowableValues": [
									                            "NFS",
									                            "CIFS",
									                            "HTTP",
									                            "HTTPS"
									                       ],
									                       "Target@Redfish.AllowableValues": [
									                            "ALL",
									                            "IDRAC",
									                            "BIOS",
									                            "NIC",
									                            "RAID"
									                       ],
									                       "ShareParameters@Redfish.AllowableValues": [
									                            "IPAddress XXXXXX
									                            "ShareName",
									                            "FileName",
									                            "UserName",
									                            "Password XXXXXX
									                            "Workgroup XXXXXX
									                            "ProxyServer XXXXXX
									                            "ProxyUserName XXXXXX
									                            "ProxyPassword XXXXXX
									                            "ProxyPort XXXXXX
									                       ]
									                  },
									                  "ExportUse@Redfish.AllowableValues": [
									                       "Default",
									                       "Clone",
									                       "Replace"
									                  ],
									                  "ExportFormat@Redfish.AllowableValues": [
									                       "XML",
									                       "JSON"
									                  ]
									             }
									        },
									   },
				*/

			}})

	// The following redfish resource is created only for the purpose of being
	// a 'receiver' for the action command specified above.
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: s.GetOdataID() + "/Actions/Manager.Reset",
			Type:        "Action",
			Context:     "Action",
			Plugin:      "GenericActionHandler",
			Privileges: map[string]interface{}{
				"POST": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{},
		},
	)

	// stream processor for action events
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction(s.GetOdataID()+"/Actions/Manager.Reset")))
	if err != nil {
		log.MustLogger("ocp_bmc").Error("Failed to create event stream processor", "err", err)
		return
	}
	sp.RunForever(func(event eh.Event) {
		log.MustLogger("ocp_bmc").Info("Got action event", "event", event)

		eventData := domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"msg": "Not Implemented"},
			StatusCode: 500,
			Headers:    map[string]string{},
		}

		handler := s.GetProperty("manager.reset")
		if handler != nil {
			if fn, ok := handler.(func(eh.Event, *domain.HTTPCmdProcessedData)); ok {
				fn(event, &eventData)
			}
		}

		responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
		eb.PublishEvent(ctx, responseEvent)
	})
}
