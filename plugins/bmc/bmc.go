package bmc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
)

var (
    BmcPlugin = domain.PluginType("obmc_bmc")
)

func init() {
	domain.RegisterInitFN(InitService)
}

// OCP Profile Redfish BMC object

type service struct {
	serviceMu sync.Mutex
	systems   map[string]bool
	chassis   map[string]bool
    mainchassis string
}

func NewService(ctx context.Context) (*service, error) {
	return &service{
		systems: map[string]bool{},
		chassis: map[string]bool{},
	}, nil
}

// wait in a listener for the root service to be created, then extend it
func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// step 1: Is this an actual openbmc?
	// TODO: add test here

	s, err := NewService(ctx)
	if err != nil {
		return
	}

    // Singleton for bmc plugin: we can pull data out of ourselves on GET/etc.
    domain.RegisterPlugin(func() domain.Plugin { return s })

	// step 2: Add openbmc manager object after Managers collection has been created
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURI("/redfish/v1/Managers"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return
	}
	sp.RunOnce(func(event eh.Event) {
		s.AddOBMCManagerResource(ctx, ch)
	})

	// we have a semi-collection of links ot systems and chassis we maintain, so add a event stream processor to keep those updated
	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURIPrefix("/redfish/v1/Systems/"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(domain.RedfishResourceCreatedData); ok {
			s.AddSystem(data.ResourceURI)
		}
	})

	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURIPrefix("/redfish/v1/Chassis/"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(domain.RedfishResourceCreatedData); ok {
			s.AddChassis(data.ResourceURI)
		}
	})

	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceRemovedByURIPrefix("/redfish/v1/Systems/"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(domain.RedfishResourceRemovedData); ok {
			s.RemoveSystem(data.ResourceURI)
		}
	})

	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceRemovedByURIPrefix("/redfish/v1/Chassis/"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(domain.RedfishResourceRemovedData); ok {
			s.RemoveChassis(data.ResourceURI)
		}
	})


	// stream processor for action events
	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction("/redfish/v1/bmc/Actions/Manager.Reset")))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		// TODO: send dbus signal to reset
		eb.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"RESET": "ok"},
			StatusCode: 200,
			Headers:    map[string]string{},
		}, time.Now()))
	})
}

// satisfy the plugin interface so we can list ourselves as a plugin in our @meta
func (s *service) PluginType() domain.PluginType { return BmcPlugin }

func (s *service) DemandBasedUpdate(
    ctx context.Context,
    agg *domain.RedfishResourceAggregate,
    rrp *domain.RedfishResourceProperty,
    method string,
    meta map[string]interface{},
    body interface{},
) {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()

    data, ok := meta["data"].(string)
    if !ok {
        fmt.Printf("Misconfigured obmc_bmc plugin: 'data' not set\n")
        return
    }

    if data == "systems" {
        list := []map[string]string{}
        for k, _  := range s.systems {
            list = append(list, map[string]string{"@odata.id": k})
        }
        list = append(list, map[string]string{"@odata.id": "/redfish/v1/Systems/"})
        rrp.Value = list
        return
    }

    if data == "chassis" {
        list := []map[string]string{}
        for k, _  := range s.chassis {
            list = append(list, map[string]string{"@odata.id": k})
        }
        rrp.Value = list
        return
    }

    if data == "mainchassis" {
        rrp.Value = map[string]string{"@odata.id": s.mainchassis}
        return
    }

    fmt.Printf("Misconfigured obmc_bmc plugin: 'data' set to something I don't know how to handle: %s\n", data)
}

// TODO: stream process for Chassis and Systems to add them to our MangerForServers and ManagerForChassis
func (s *service) AddSystem(uri string) {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()
	fmt.Printf("DEBUG: ADDING SYSTEM(%s) to list: %s\n", uri, s.systems)
	s.systems[uri] = true
}

func (s *service) RemoveSystem(uri string) {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()
	fmt.Printf("DEBUG: REMOVING SYSTEM(%s) to list: %s\n", uri, s.systems)
    delete(s.systems, uri)
}

func (s *service) AddChassis(uri string) {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()
    if s.mainchassis == "" {
        s.mainchassis = uri
    }
	fmt.Printf("DEBUG: ADDING CHASSIS(%s) to list: %s\n", uri, s.chassis)
	s.chassis[uri] = true
}

func (s *service) RemoveChassis(uri string) {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()
    if s.mainchassis == uri {
        s.mainchassis = ""
    }
	fmt.Printf("DEBUG: REMOVING CHASSIS(%s) to list: %s\n", uri, s.chassis)
    delete(s.chassis, uri)
}

func (s *service) AddOBMCManagerResource(ctx context.Context, ch eh.CommandHandler) {
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
					"ManagerForServers@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "data": "systems"}},
					"ManagerForChassis@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "data": "chassis"}},
					"ManagerInChassis@meta":  map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "data": "mainchassis"}},
				},
				"Actions": map[string]interface{}{
					"#Manager.Reset": map[string]interface{}{
						"target": "/redfish/v1/Managers/bmc/Actions/Manager.Reset",
						"ResetType@Redfish.AllowableValues": []string{
							"ForceRestart",
							"GracefulRestart",
						},
					},
				},
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
