package bmc

import (
	"context"
	"time"

	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
)

func init() {
	domain.RegisterInitFN(InitService)
	domain.RegisterInitFN(ResetActionHandler)
}

// OCP Profile Redfish BMC object
//

// wait in a listener for the root service to be created, then extend it
func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// step 1: Is this an actual openbmc?

	// step 2: Add openbmc manager object after Managers collection has been created
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURI("/redfish/v1/Managers"))
	if err == nil {
		sp.RunOnce(func(event eh.Event) {
			NewService(ctx, ch)
		})
	}

	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURIPrefix("/redfish/v1/Systems/"))
	if err == nil {
		sp.RunForever(func(event eh.Event) {
			MaintainManagersForSystems(ctx, ch)
		})
	}

	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURIPrefix("/redfish/v1/Chassis/"))
	if err == nil {
		sp.RunForever(func(event eh.Event) {
			MaintainManagersForChassis(ctx, ch)
		})
	}
}

func MaintainManagersForSystems(ctx context.Context, ch eh.CommandHandler) {
}

func MaintainManagersForChassis(ctx context.Context, ch eh.CommandHandler) {
}

func NewService(ctx context.Context, ch eh.CommandHandler) {
	// TODO: stream process for Chassis and Systems to add them to our MangerForServers and ManagerForChassis
	// TODO: set up Action links

	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  false,
			ResourceURI: "/redfish/v1/Managers/bmc",
			Type:        "#Manager.v1_1_0.Manager",
			Context:     "/redfish/v1/$metadata#Manager.Manager",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":                    "bmc",
				"Name":                  "Manager",
				"ManagerType":           "BMC",
				"Description":           "BMC",
				"ServiceEntryPointUUID": eh.NewUUID(),
				"UUID":                  eh.NewUUID(),
				"Model":                 "CatfishBMC",
				"DateTime@meta":         map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
				"DateTimeLocalOffset":   "+06:00",
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"FirmwareVersion":    "1.00",
				"NetworkProtocol":    map[string]interface{}{"@odata.id": "/redfish/v1/Managers/bmc/NetworkProtocol"},
				"EthernetInterfaces": map[string]interface{}{"@odata.id": "/redfish/v1/Managers/bmc/EthernetInterfaces"},
				"Links": map[string]interface{}{
					"ManagerForServers": []interface{}{
						map[string]interface{}{"@odata.id": "/redfish/v1/Systems/"},
					},
					"ManagerForChassis": []interface{}{},
					"ManagerInChassis":  map[string]interface{}{},
					"Oem":               map[string]interface{}{},
				},
				"Actions": map[string]interface{}{
					"#Manager.Reset": map[string]interface{}{
						"target": "/redfish/v1/Managers/bmc/Actions/Manager.Reset",
						"ResetType@Redfish.AllowableValues": []string{
							"ForceRestart",
							"GracefulRestart",
						},
					},
					"Oem": map[string]interface{}{},
				},
				"Oem": map[string]interface{}{},
			}})

	// handle action for restart
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: "/redfish/v1/bmc/Actions/Manager.Reset",
			Type:        "Action",
			Context:     "Action",
			Plugin:      "GenericActionHandler",
			Privileges: map[string]interface{}{
				"POST": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{},
		},
	)

	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  false,
			ResourceURI: "/redfish/v1/Managers/bmc/NetworkProtocol",
			Type:        "#ManagerNetworkProtocol.v1_0_2.ManagerNetworkProtocol",
			Context:     "/redfish/v1/$metadata#ManagerNetworkProtocol.ManagerNetworkProtocol",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":          "NetworkProtocol",
				"Name":        "Manager Network Protocol",
				"Description": "Manager Network Service Status",
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"HostName@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "hostname"}},
				"FQDN":          "mymanager.mydomain.com",
				"HTTP": map[string]interface{}{
					"ProtocolEnabled": false,
					"Port":            80,
				},
				"HTTPS": map[string]interface{}{
					"ProtocolEnabled": true,
					"Port":            443,
				},
				"IPMI": map[string]interface{}{
					"ProtocolEnabled": false,
					"Port":            623,
				},
				"SSH": map[string]interface{}{
					"ProtocolEnabled": false,
					"Port":            22,
				},
				"SNMP": map[string]interface{}{
					"ProtocolEnabled": false,
					"Port":            161,
				},
				"SSDP": map[string]interface{}{
					"ProtocolEnabled": false,
					"Port":            1900,
					"NotifyMulticastIntervalSeconds": 600,
					"NotifyTTL":                      5,
					"NotifyIPv6Scope":                "Site",
				},
				"Telnet": map[string]interface{}{
					"ProtocolEnabled": false,
					"Port":            23,
				},
			}})

	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: true,

			ResourceURI: "/redfish/v1/Managers/bmc/EthernetInterfaces",
			Type:        "#EthernetInterfaceCollection.EthernetInterfaceCollection",
			Context:     "/redfish/v1/$metadata#EthernetInterfaceCollection.EthernetInterfaceCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name": "Ethernet Network Interface Collection",
			}})
}

func ResetActionHandler(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// step 2: Add openbmc manager object after Managers collection has been created
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction("/redfish/v1/bmc/Actions/Manager.Reset")))
	if err == nil {
		sp.RunForever(func(event eh.Event) {
			eb.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
				CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
				Results:    map[string]interface{}{"RESET": "ok"},
				StatusCode: 200,
				Headers:    map[string]string{},
			}, time.Now()))
		})
	}
}
