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

func (s *service) AddView(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          plugins.GetUUID(s),
			Collection:  false,
			ResourceURI: plugins.GetOdataID(s),
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
				"Name@meta":                s.Meta(plugins.PropGETOptional("name")),
				"ManagerType":              "BMC",
				"Description@meta":         s.Meta(plugins.PropGETOptional("description")),
				"Model@meta":               s.Meta(plugins.PropGETOptional("model")),
				"DateTime@meta":            map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
				"DateTimeLocalOffset@meta": s.Meta(plugins.PropGETOptional("timezone"), plugins.PropPATCHOptional("timezone")),
				"FirmwareVersion@meta":     s.Meta(plugins.PropGETOptional("firmware_version")),
				"Links": map[string]interface{}{
					"ManagerForServers@meta": s.Meta(plugins.PropGET("bmc_manager_for_servers")),
					// TODO: Need standard method to count arrays
					// "ManagerForChassis@odata.count": 1,
					"ManagerForChassis@meta": s.Meta(plugins.PropGET("bmc_manager_for_chassis")),
					"ManagerInChassis@meta":  s.Meta(plugins.PropGET("in_chassis")),
				},

				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State@meta":   s.Meta(plugins.PropGETOptional("health_state")),
					"Health":       "OK",
				},

				"Redundancy@odata.count": 1,
				"Redundancy": []interface{}{
					map[string]interface{}{
						"@odata.type": "#Redundancy.v1_0_2.Redundancy",
						"Status": map[string]interface{}{
							"HealthRollup": "OK",
							"State@meta":   s.Meta(plugins.PropGETOptional("redundancy_health_state")),
							"Health":       "OK",
						},
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
						"Mode@meta":                 s.Meta(plugins.PropGETOptional("redundancy_mode")),
						"MinNumNeeded@meta":         s.Meta(plugins.PropGETOptional("redundancy_min")),
						"MaxNumSupported@meta":      s.Meta(plugins.PropGETOptional("redundancy_max")),
					},
				},
				"SerialConsole": map[string]interface{}{
					"ConnectTypesSupported@odata.count": 0,
					"MaxConcurrentSessions":             0,
					"ConnectTypesSupported":             []interface{}{},
					"ServiceEnabled":                    false,
				},

				"CommandShell": map[string]interface{}{
					"ConnectTypesSupported@odata.count": 0,
					"MaxConcurrentSessions":             0,
					"ConnectTypesSupported":             []interface{}{},
					"ServiceEnabled":                    false,
				},

				"LogServices": map[string]interface{}{
					"@odata.id": "/redfish/v1/Managers/CMC.Integrated.1/LogServices",
				},

				"GraphicalConsole": map[string]interface{}{
					"ConnectTypesSupported@odata.count": 0,
					"MaxConcurrentSessions":             0,
					"ConnectTypesSupported":             []interface{}{},
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

				"Actions": map[string]interface{}{
					"#Manager.Reset": map[string]interface{}{
						"target": plugins.GetOdataID(s) + "/Actions/Manager.Reset",
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
					},
				},

				/*
					******************************************************
					 DISABLED FOR NOW because the output is mangled by dumplogs
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

	logger, _ := log.GetLogger("Managers/CMC.Integrated.1")

	CreateAction(ctx, ch, eb, ew,
		logger,
		plugins.GetOdataID(s)+"/Actions/Manager.Reset",
		"manager.reset",
		s)

	CreateAction(ctx, ch, eb, ew,
		logger,
		plugins.GetOdataID(s)+"/Actions/Oem/DellManager.ResetToDefaults",
		"manager.resettodefaults",
		s)

	CreateAction(ctx, ch, eb, ew,
		logger,
		plugins.GetOdataID(s)+"/Actions/Manager.ForceFailover",
		"manager.forcefailover",
		s)
}

type prop interface {
	GetProperty(string) interface{}
}

func CreateAction(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter,
	logger log.Logger,
	uri string,
	property string,
	s prop,
) {
	// The following redfish resource is created only for the purpose of being
	// a 'receiver' for the action command specified above.
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: uri,
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
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction(uri)))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return
	}
	sp.RunForever(func(event eh.Event) {
		eventData := domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"msg": "Not Implemented"},
			StatusCode: 500,
			Headers:    map[string]string{},
		}

		handler := s.GetProperty(property)
		if handler != nil {
			if fn, ok := handler.(func(eh.Event, *domain.HTTPCmdProcessedData)); ok {
				fn(event, &eventData)
			}
		} else {
			logger.Warn("UNHANDLED action event: no function handler set up for this event.", "event", event)
		}

		responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
		eb.PublishEvent(ctx, responseEvent)
	})
}
